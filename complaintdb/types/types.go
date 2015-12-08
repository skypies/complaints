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
}

// Attempt to split into firstname, surname
func (p ComplainerProfile)SplitName() (first,last string) {
	str := strings.Replace(p.FullName, "\n", " ", -1)
	str = strings.TrimSpace(str)
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
	return fmt.Sprintf("<%s>@%s [%-6.6s] %d \"%s\"",
		c.Profile.EmailAddress,
		c.Timestamp,
		c.AircraftOverhead.FlightNumber,
		c.Loudness,
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
