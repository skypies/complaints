package flightid

import(
	"fmt"
	"regexp"
	"time"
	"github.com/skypies/geo"
	fdb "github.com/skypies/flightdb"
)

type Aircraft struct {
	//`datastore:"-"` // for all these ??
	Dist                float64  `datastore:",noindex"`// in KM
	Dist3               float64  `datastore:",noindex"`// in KM (3D dist, taking height into account)
	BearingFromObserver float64  `datastore:",noindex"`// bearing from the house
	Fr24Url             string   `datastore:",noindex"`// Flightradar's playback view
	
	Id string            `datastore:",noindex"`// Our ID for this instance of this flight
	Id2 string                                 // Better known as ModeS
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

type AircraftByDist3 []Aircraft
func (s AircraftByDist3) Len() int      { return len(s) }
func (s AircraftByDist3) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s AircraftByDist3) Less(i, j int) bool { return s[i].Dist3 < s[j].Dist3 }

func (a Aircraft) String() string {
	return fmt.Sprintf("%s[%s:%s-%s]", a.Id, a.FlightNumber, a.Origin, a.Destination)
}

func (a Aircraft)Latlong() geo.Latlong { return geo.Latlong{Lat:a.Lat, Long:a.Long} }

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

func (a Aircraft)Trackpoint() fdb.Trackpoint {
	return fdb.Trackpoint{
		Latlong: geo.Latlong{a.Lat, a.Long},
		Heading: a.Track,
		Altitude: a.Altitude,
		GroundSpeed: a.Speed,
		ReceiverName: a.Radar,
		TimestampUTC: time.Unix(int64(a.Epoch), 0),
		VerticalRate: a.VerticalSpeed,
	}
}

func (a *Aircraft)FromTrackpoint(tp fdb.Trackpoint) {
	a.Lat = tp.Latlong.Lat
	a.Long = tp.Latlong.Long
	a.Track = tp.Heading
	a.Altitude = tp.Altitude
	a.Speed = tp.GroundSpeed
	a.Radar = tp.ReceiverName
	a.Epoch = float64(tp.TimestampUTC.Unix())
	a.VerticalSpeed = tp.VerticalRate
}
