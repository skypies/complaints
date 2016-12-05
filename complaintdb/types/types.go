package types

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
	
	"github.com/skypies/geo"
	fdb "github.com/skypies/flightdb2"

	"github.com/skypies/complaints/flightid"
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
	CcSfo             bool `datastore:",noindex"`

	DataSharing       int  // 0 == unset, 1 == OK/yes, -1 == no
	ThirdPartyComms   int  `datastore:",noindex"` // 0 == unset, 1 == OK/yes, -1 == no

	ButtonId        []string // AWS IoT button serial numbers
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

func (p ComplainerProfile)GetStructuredAddress() PostalAddress {
	addr := p.StructuredAddress

	if addr.Zip == "" {
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


func (p ComplainerProfile)DataSharingOK() bool {
	return p.DataSharing >= 0 // The default is "yes"
}

func (p ComplainerProfile)ThirdPartyCommsOK() bool {
	return p.ThirdPartyComms >= 0 // The default is "yes"
}


//	DataSharingOK     int  // 0 == unknown, 1 == yes, -1 == no
//	ThirdPartyComssOK int  // 0 == unknown, 1 == yes, -1 == no

// }}}
// {{{ Submission{}

type SubmissionOutcome int
const(
	SubmissionNotAttempted SubmissionOutcome = iota
	SubmissionAccepted
	SubmissionFailed
)
func (so SubmissionOutcome)String() string {
	switch so {
	case SubmissionNotAttempted: return "tbd"
	case SubmissionAccepted: return "OK"
	case SubmissionFailed: return "fail"
	default: return fmt.Sprintf("?%d?", so)
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

func (s Submission)String() string {
	return fmt.Sprintf("{%s@%s(%d)-%s:%db}", s.Outcome, s.T, s.Attempts, s.D, len(s.Key))
}

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
		url := fmt.Sprintf("http://fdb.serfr1.org/fdb/descent?idspec=%s&dist=from&classb=1&refpt_lat=%.6f&refpt_long=%.6f&refpt_label=You", id, c.Profile.Lat, c.Profile.Long)

		return template.HTML(fmt.Sprintf("<a target=\"_blank\" href=\"%s\">%s</a>", url, txt))
	}
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
