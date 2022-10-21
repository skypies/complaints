package types

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"
	
	"github.com/skypies/geo"
	fdb "github.com/skypies/flightdb"
	"github.com/skypies/complaints/config"
	"github.com/skypies/complaints/flightid"

	"golang.org/x/net/context"

	"googlemaps.github.io/maps"
)

// The various types used in the complaint db

// {{{ PostalAddress{}

type PostalAddress struct {
	Number string `datastore:",noindex"`
	Street string `datastore:",noindex"`
	City string
	State string `datastore:",noindex"`
	Zip string
	Country string `datastore:",noindex"`
}

// }}}
// {{{ ComplainerProfile{}

// A ComplainerProfile represents a single human who makes complaints, and has a caller code
// This is the root object for the datastore; the individual complaints hang off this
type ComplainerProfile struct {
	EmailAddress      string // This is the root key; we get it from the GAE user profile.
	CallerCode        string
	FullName          string `datastore:",noindex"`
	Address           string `datastore:",noindex"`
	StructuredAddress PostalAddress
	Lat,Long          float64
	Elevation         float64 // in meters
	CcSfo             bool `datastore:",noindex"`
	SelectorAlgorithm string  // Users can have different algorithms (and maybe even params someday)

	SendDailyEmail    int  // 0 == unset, 1 == OK/yes, -1 == no
	DataSharing       int  // 0 == unset, 1 == OK/yes, -1 == no
	ThirdPartyComms   int  `datastore:",noindex"` // 0 == unset, 1 == OK/yes, -1 == no

	ButtonId        []string // AWS IoT button serial numbers
}
func (p ComplainerProfile)ElevationFeet() float64 {
	elevKM := p.Elevation / 1000.0
	return elevKM * geo.KFeetPerKM
}

// Attempt to split into firstname, surname
func (p ComplainerProfile)SplitName() (first,last string) {
	str := strings.Replace(p.FullName, "\n", " ", -1)
	str = strings.TrimSpace(str)

	if str == "" { return "No", "Name" }
	
	words := strings.Split(str, " ")
	if len(words) == 1 {
		first,last = str,str
	} else {
		last,words = words[len(words)-1], words[:len(words)-1]
		first = strings.Join(words," ")
	}
	return
}

var towns = []string{
	"aptos", "soquel", "capitola", "santa cruz", "scotts valley",
	"glenwood", "los gatos", "palo alto",
}

func (p ComplainerProfile)Base64Encode() (string, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(p); err != nil {
		return "", err
	} else {
		return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	}
}
func (p *ComplainerProfile)Base64Decode(str string) error {
	if data,err := base64.StdEncoding.DecodeString(str); err != nil {
		return err
	} else {
		buf := bytes.NewBuffer(data)
		err := gob.NewDecoder(buf).Decode(&p)
		return err
	}
}

func (p ComplainerProfile)SendDailyEmailOK() bool {
	return p.SendDailyEmail >= 0 // The default is "yes"
}
func (p ComplainerProfile)DataSharingOK() bool {
	return p.DataSharing >= 0 // The default is "yes"
}
func (p ComplainerProfile)ThirdPartyCommsOK() bool {
	return p.ThirdPartyComms >= 0 // The default is "yes"
}

// }}}

// {{{ p.GetStructuredAddress

func (p *ComplainerProfile)GetStructuredAddress() PostalAddress {
	// Don't call the map geocoder on every access to this call - costs real money :/
	// if p.StructuredAddress.Street == "" && p.Address != "" {
	//	p.UpdateStructuredAddress()
	// }
	return p.StructuredAddress

/*
	addr := p.StructuredAddress

	if p.StructuredAddress.Zip == "" {
		// Bodge it up
		addr.Street = p.Address
		addr.State  = "CA"
		addr.Zip    = regexp.MustCompile("(?i:(\\d{5}(-\\d{4})?))").FindString(p.Address)
		addr.City   = regexp.MustCompile("(?i:"+strings.Join(towns,"|")+")").FindString(p.Address)
		if addr.City == "" { addr.City  = "Unknown" }
		if addr.Zip  == "" { addr.Zip   = "99999" }

		addr.City = strings.Title(addr.City)

	} else {
		if addr.Number != "" {
			addr.Street = addr.Number + " " + addr.Street
		}
	}
	
	return addr
*/
}

// }}}
// {{{ p.UpdateStructuredAddress

func components2structured (r maps.GeocodingResult) PostalAddress {
	new := PostalAddress{}
	for _,c := range r.AddressComponents {
		switch c.Types[0] {
		case "street_number": new.Number = c.LongName
		case "route": new.Street = c.LongName
		case "locality": new.City = c.LongName
		case "administrative_area_level_1": new.State = c.LongName
		case "postal_code":  new.Zip = c.LongName
		case "country":  new.Country = c.LongName
		}
	}
	return new
}

func (p *ComplainerProfile)FetchStructuredAddress() (PostalAddress, error) {
	ctx := context.Background()

	apiKey := config.Get("googlemaps.apikey.serverside")

	if mapsClient, err := maps.NewClient(maps.WithAPIKey(apiKey)); err != nil {
		return PostalAddress{}, fmt.Errorf("NewClient err: %v\n", err)
	} else {
		// https://godoc.org/googlemaps.github.io/maps#GeocodingRequest
		req := &maps.GeocodingRequest{Address: p.Address}
		if results,err := mapsClient.Geocode(ctx, req); err != nil {
			return PostalAddress{}, fmt.Errorf("maps.Geocode err: %v", err)
		} else {
			if len(results) != 1 {
				return PostalAddress{}, fmt.Errorf("maps.Geocode: %d results (%s)", len(results), p.Address)
			}
			if results[0].PartialMatch {
				return PostalAddress{}, fmt.Errorf("maps.Geocode: partial match (%s)", p.Address)
			}
			return components2structured(results[0]), nil
		}
	}
}


func (p *ComplainerProfile)UpdateStructuredAddress() error {
	if newAddr,err := p.FetchStructuredAddress(); err != nil {
		return err
	} else {
		p.StructuredAddress = newAddr
	}

	return nil
}

// }}}

// {{{ Submission{}

type SubmissionOutcome int
const(
	SubmissionNotAttempted SubmissionOutcome = iota
	SubmissionAccepted
	SubmissionTimeout
	SubmissionRejected
	SubmissionFailed
)
func (so SubmissionOutcome)String() string {
	switch so {
	case SubmissionNotAttempted: return "tbd"
	case SubmissionAccepted: return "OK"
	case SubmissionTimeout: return "timeout"
	case SubmissionRejected: return "reject"
	case SubmissionFailed: return "fail"
	default: return fmt.Sprintf("?%d?", so)
	}
}

type SubmissionRejectReason int
const(
	SubmissionNoReject = iota
	SubmissionRejectConstraint
	SubmissionRejectDupeReceipt
	SubmissionRejectBadField
	SubmissionRejectBadApiKey
	SubmissionRejectOther
)
func (srr SubmissionRejectReason)String() string {
	switch srr {
	case SubmissionNoReject: return "OK"
	case SubmissionRejectConstraint: return "db-constraint"
	case SubmissionRejectDupeReceipt: return "dupe-receipt"
	case SubmissionRejectBadField: return "bad-field"
	case SubmissionRejectBadApiKey: return "bad-apikey"
	case SubmissionRejectOther: return "unclassified"
	default: return fmt.Sprintf("?%d?", srr)
	}
}


// Fields about backend submission
type Submission struct {
	T            time.Time
	D            time.Duration // Of most recent submission
	Outcome      SubmissionOutcome
	Response   []byte      `datastore:",noindex"` // JSON response, in full
	Key          string    `datastore:",noindex"` // Foreign key, from backend
	Attempts     int
	Log          string    `datastore:",noindex"`
}

func (s Submission)WasFailure() bool {
	return s.Outcome == SubmissionTimeout || s.Outcome == SubmissionRejected || s.Outcome == SubmissionFailed
}

func (s Submission)String() string {
	str := s.Outcome.String()
	if s.Outcome == SubmissionRejected {
		srr,txt := s.ClassifyRejection()
		str += "/" + srr.String()
		if srr == SubmissionRejectBadField {
			str += "-" + txt
		}
	}
	return fmt.Sprintf("{%s@%s(%d)-%s:%db}", str, s.T, s.Attempts, s.D, len(s.Key))
}

// {{{ s.ClasifyRejection

func (s Submission)ClassifyRejection() (SubmissionRejectReason, string) {
	if s.Outcome != SubmissionRejected {
		return SubmissionNoReject, "not rejected"
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(s.Response, &jsonMap); err != nil {
		return SubmissionRejectOther, "could not parse response as json"
	}

	/* Look for backend errors of this form:
      {
        "body": "\u003cp\u003eYour noise report was not submitted because there was a problem with this online noise report system. Try submitting the noise report again or contact us by email at noiseabatementoffice@flysfo.com\u003c/p\u003e\n",
        "debug": "\nsubmittingError inserting into database.Cannot add or update a child row: a foreign key constraint fails (`sfo5`.`submissions`, CONSTRAINT `complaints_fkey5` FOREIGN KEY (`browser_id`) REFERENCES `browsers` (`browser_id`) ON DELETE NO ACTION ON UPDATE NO ACTION)",
        "error": "Error inserting into database.Cannot add or update a child row: a foreign key constraint fails (`sfo5`.`submissions`, CONSTRAINT `complaints_fkey5` FOREIGN KEY (`browser_id`) REFERENCES `browsers` (`browser_id`) ON DELETE NO ACTION ON UPDATE NO ACTION)",
        "result": "0",
        "title": "Error"
      }
  */
	if v := jsonMap["error"]; v != nil {
		e := v.(string)
		switch {
		case strings.Contains(e, "key constraint fails"): //regexp.MustCompile("").MatchString(e):
			// "Error inserting into database.Cannot add or update a child
			// row: a foreign key constraint fails (`sfo5`.`submissions`,
			// CONSTRAINT `complaints_fkey5` FOREIGN KEY (`browser_id`)
			// REFERENCES `browsers` (`browser_id`) ON DELETE NO ACTION ON
			// UPDATE NO ACTION)"
			return SubmissionRejectConstraint, e

		case strings.Contains(e, "receipt_key_UNIQUE"):
			// "Error inserting into database.Duplicate entry
			// 'asdasdadasdasdasdasdasdasda' for key 'receipt_key_UNIQUE'
			return SubmissionRejectDupeReceipt, e

		case strings.Contains(e, "validateSubmitKey"):
			// "validateSubmitKey returned false"
			return SubmissionRejectBadApiKey, e

		default:
			return SubmissionRejectOther, e
		}
	}

	/* Look for problems with mandatory fields
{
  "body": "There are some problems. Please correct the mistakes and submit the form again.",
  "debug": "",
  "required": [
    "activity_type",
    "address1",
    "aircrafttype",
    "city",
    "date",
    "event_type",
    "name",
    "surname",
    "time"
  ],
  "result": "0",
  "site_name": "sfo5",
  "submitted": {
    "activity_type": {
      "error": "",
      "formatted": null,
      "ok": true,
      "value": "Other"
    },
    "address1": {
      "error": "Please fill in",
      "formatted": null,
      "ok": false,
      "value": ""
    }
  */
	if v := jsonMap["submitted"]; v != nil {
		// If we found this 'submitted' element, we're almost certainly in a bad field response.
		probs := []string{}
		if w,ok := jsonMap["submitted"].(map[string]interface{}); ok { // cast it to a map
			for name,vals := range w {
				if m,ok := vals.(map[string]interface{}); ok {
					if m["ok"] == false {
						probs = append(probs, name)
					}
				}
			}
		}

		return SubmissionRejectBadField, fmt.Sprintf("[%s]", strings.Join(probs, ","))
	}

	return SubmissionRejectOther, "could not classify"
}

// }}}

// }}}
// {{{ Browser{}

// Fields about backend submission
type Browser struct {
	UUID, Name, Version, Vendor, Platform string `datastore:",noindex"`
}

func (b Browser)String() string {
	return fmt.Sprintf("{%sv%s/%s/%s %s}", b.Name, b.Version, b.Platform, b.Vendor, b.UUID)
}

// }}}

// {{{ Complaint{}

type Complaint struct {
	Version          int           `datastore:",noindex"` // undef or 0 means unversioned thingy.
	Description      string        `datastore:",noindex"`
	Timestamp        time.Time
	AircraftOverhead flightid.Aircraft
	Debug            string        `datastore:",noindex"` // Debugging; mostly about flight lookup

	HeardSpeedbreaks bool
	Loudness         int           `datastore:",noindex"` // 0=undef, 1=loud, 2=very loud, 3=insane
	Activity         string        `datastore:",noindex"` // What was disturbed

	Profile          ComplainerProfile                    // Embed the whole profile

	Submission       // embedded; details about submitting the complaint to a backend
	Browser          Browser // Details about the browser used
	
	// Synthetic fields
	DatastoreKey     string        `datastore:"-"`
	Dist2KM          float64       `datastore:"-"`        // Distance from home to aircraft
	Dist3KM          float64       `datastore:"-"`
}

type ComplaintsByTimeDesc []Complaint
func (a ComplaintsByTimeDesc) Len() int           { return len(a) }
func (a ComplaintsByTimeDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ComplaintsByTimeDesc) Less(i, j int) bool { return a[i].Timestamp.After(a[j].Timestamp) }

func (c Complaint) String() string {
	return fmt.Sprintf("{%s}@%s [%-6.6s] %d %s \"%s\"",
		c.Profile.EmailAddress,
		c.Timestamp,
		c.AircraftOverhead.FlightNumber,
		c.Loudness,
		c.Submission.String(),
		c.Description,
	)
}

func (c Complaint) AltitudeHrefString() template.HTML {
	txt := fmt.Sprintf("%.0fft", c.AircraftOverhead.Altitude)

	if id,err := fdb.NewIdSpec(c.AircraftOverhead.Id); err != nil {
		return template.HTML(txt)
	} else {
		url := fmt.Sprintf("http://fdb.serfr1.org/fdb/sideview?idspec=%s&dist=from&classb=1&refpt_lat=%.6f&refpt_long=%.6f&refpt_label=You", id, c.Profile.Lat, c.Profile.Long)

		return template.HTML(fmt.Sprintf("<a target=\"_blank\" href=\"%s\">%s</a>", url, txt))
	}
}

// }}}
// {{{ c.ToCopyWithStoredDataOnly

// ToCopyWithStoredDataOnly returns a copy of the complaint that only has the stored data fields
// (e.g. no synthetic fields). This is for use when archiving, and verifying archives.
func (c1 Complaint)ToCopyWithStoredDataOnly() Complaint {
	c2 := c1

	c2.DatastoreKey = ""
	c2.Dist2KM = 0.0
	c2.Dist3KM = 0.0

	return c2
}

// }}}

// {{{ CountItem{}

// This is just maddening boilerplate junk we need
type CountItem struct {
	Key string
	Count int
	TotalComplaints int
	TotalComplainers int
	IsMaxComplaints bool
	IsMaxComplainers bool
}
type countItemSlice []CountItem
func (a countItemSlice) Len() int           { return len(a) }
func (a countItemSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a countItemSlice) Less(i, j int) bool { return a[i].Key > a[j].Key }

// }}}
// {{{ ComplaintsAndProfile{}

type ComplaintsAndProfile struct {
	Profile ComplainerProfile
	Complaints []Complaint
	Counts []CountItem
}

// }}}

// {{{ AnonymizedComplaint{}

// AnonymizedComplaint is a subset of the data from a complaint; these values are released to the
// public as a queryable database. The data is somewhat denormalized, to allow for efficient
// indexing & querying.
type AnonymizedComplaint struct {
	Timestamp        time.Time
	Speedbrakes      bool
	Loudness         int
	Activity         string

	User             string // A hash fingerprint of the email address
	City             string
	Zip              string

	// Denormalized fields to index/group by
	DatePST          string
	HourPST          int
	
	// Aircraft details; might be null
	FlightKey        string // Flightnumber plus date (e.g. "UA123-20161231") - to allow for joins
	FlightNumber     string // IATA scheduled flight number
	AirlineCode      string // The 2-char IATA airline code
	
	Origin           string
	Destination      string
	EquipType        string // B744, etc

	geo.Latlong      // embedded; location of aircraft at Timestamp
	PressureAltitude float64
	Groundspeed      float64
}

func (ac AnonymizedComplaint)HasIdenitifiedAircraft() bool {
	return ac.FlightNumber != ""
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
