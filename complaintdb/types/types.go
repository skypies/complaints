package types

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"regexp"
	"strings"
	"time"
	
	"github.com/skypies/complaints/fr24"
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
	Lat,Long          float64 `datastore:",noindex"`
	CcSfo             bool `datastore:",noindex"`

	DataSharing       int  `datastore:",noindex"` // 0 == unset, 1 == OK/yes, -1 == no
	ThirdPartyComms   int  `datastore:",noindex"` // 0 == unset, 1 == OK/yes, -1 == no
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

// {{{ Complaint{}

type Complaint struct {
	Version          int           `datastore:",noindex"` // undef or 0 means unversioned thingy.
	Description      string        `datastore:",noindex"`
	Timestamp        time.Time
	AircraftOverhead fr24.Aircraft `datastore:",noindex"`
	Debug            string        `datastore:",noindex"` // Debugging; mostly about flight lookup

	HeardSpeedbreaks bool
	Loudness         int           `datastore:",noindex"` // 0=undef, 1=loud, 2=very loud, 3=insane
	Activity         string        `datastore:",noindex"` // What was disturbed

	Profile          ComplainerProfile                    // Embed the whole profile

	Submission       // embedded; details about submitting the complaint to a backend
	
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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
