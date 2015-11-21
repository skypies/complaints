package types

import (
	"regexp"
	"strings"
	"time"
	
	"github.com/skypies/complaints/fr24"
)

// The various types used in the complaint db

// {{{ PostalAddress{}

type PostalAddress struct {
	Number, Street, City, State, Zip, Country string
}

// }}}
// {{{ ComplainerProfile{}

// A ComplainerProfile represents a single human who makes complaints, and has a caller code
// This is the root object for the datastore; the individual complaints hang off this
type ComplainerProfile struct {
	EmailAddress      string // This is the root key; we get it from the GAE user profile.
	CallerCode        string
	FullName          string
	Address           string
	StructuredAddress PostalAddress
	Lat,Long          float64
	CcSfo             bool
}

// Attempt to split into firstname, surname
func (p ComplainerProfile)SplitName() (first,last string) {
	words := strings.Split(p.FullName, " ")
	if len(words) == 1 {
		first,last = p.FullName,p.FullName
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

	} else {
		if addr.Number != "" {
			addr.Street = addr.Number + " " + addr.Street
		}
	}

	return addr
}

// }}}

// {{{ Complaint{}

type Complaint struct {
	Version          int           `datastore:",noindex"` // undef or 0 means unversioned thingy.
	Description      string        `datastore:",noindex"`
	Timestamp        time.Time
	AircraftOverhead fr24.Aircraft `datastore:",noindex"`
	Debug            string        `datastore:",noindex"` // Debugging; mostly about flight lookup

	// Synthetic fields
	DatastoreKey     string        `datastore:"-"`

	// Fields added in v2
	HeardSpeedbreaks bool
	Loudness         int           `datastore:",noindex"` // 0=undefined, 1=loud, 2=very loud, 3=insane
	Activity         string        `datastore:",noindex"` // What was disturbed
}

type ComplaintsByTimeDesc []Complaint
func (a ComplaintsByTimeDesc) Len() int           { return len(a) }
func (a ComplaintsByTimeDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ComplaintsByTimeDesc) Less(i, j int) bool { return a[i].Timestamp.After(a[j].Timestamp) }

// }}}

// {{{ CountItem{}

// This is just maddening boilerplate junk we need
type CountItem struct {
	Key string
	Count int
	TotalComplaints int
	TotalComplainers int
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
