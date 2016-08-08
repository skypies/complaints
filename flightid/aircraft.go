package flightid

import(
	"fmt"
	"regexp"
)

type Aircraft struct {
	//`datastore:"-"` // for all these ??
	Dist                float64  `datastore:",noindex"`// in KM
	Dist3               float64  `datastore:",noindex"`// in KM (3D dist, taking height into account)
	BearingFromObserver float64  `datastore:",noindex"`// bearing from the house
	Fr24Url             string   `datastore:",noindex"`// Flightradar's playback view
	
	Id string            `datastore:",noindex"`// Flightradar's ID for this instance of this flight
	Id2 string           `datastore:",noindex"`// Better known as ModeS
	Lat float64 `datastore:",noindex"`
	Long float64 `datastore:",noindex"`
	Track float64 `datastore:",noindex"`

	Altitude float64 `datastore:",noindex"`
	Speed float64 `datastore:",noindex"`
	Squawk string `datastore:",noindex"`
	Radar string `datastore:",noindex"`
	EquipType string `datastore:",noindex"`
	
	Registration string `datastore:",noindex"`
	Epoch float64 `datastore:",noindex"`
	Origin string `datastore:",noindex"`
	Destination string `datastore:",noindex"`
	FlightNumber string
	
	Unknown float64 `datastore:",noindex"`
	VerticalSpeed float64 `datastore:",noindex"`
	Callsign string `datastore:",noindex"`
	Unknown2 float64 `datastore:",noindex"`
}

func (a Aircraft) String() string {
	return fmt.Sprintf("%s[%s:%s-%s]", a.Id, a.FlightNumber, a.Origin, a.Destination)
}

// Why is this not getting invoked correctly ?/
func (a Aircraft)BestIdent() string {
	if a.FlightNumber != ""      {
		return a.FlightNumber
	} else if a.Registration != "" {
		return "r:"+a.Registration
	}
	return ""
}

func (a Aircraft)IATAAirlineCode() string {
	// Stolen from flightdb2/identity.go
	iata := regexp.MustCompile("^([A-Z][0-9A-Z])([0-9]{1,4})$").FindStringSubmatch(a.FlightNumber)
	if iata != nil && len(iata)==3 {
		return iata[1]
	}

	return ""
}
