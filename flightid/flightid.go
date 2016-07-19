package flightid

import(
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	//"golang.org/x/net/context"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/pi/airspace"
	fdb "github.com/skypies/flightdb2"

	"github.com/skypies/complaints/fr24"
)

func init() {
	http.HandleFunc("/cdb/airspace", airspaceHandler)
}

// {{{ airspaceHandler

func airspaceHandler(w http.ResponseWriter, r *http.Request) {
	client := urlfetch.Client(appengine.NewContext(r))

	str := ""
	
	f,err,debug := FindOverhead(client, geo.Latlong{37.060312,-121.990814}, 0, true)
	str += fmt.Sprintf("======/// FindOverhead ///======\nerr=%v\nf=%s\n\n%s", err, f, debug)

/*
	as, err := FetchAirspace(client)
	if err != nil {
		http.Error(w, fmt.Sprintf("FetchAirspace: %v", err), http.StatusInternalServerError)
		return
	}
	str += "======/// Airspace ///=====\n\n" + as.String()

	snaps := AirspaceToSnapshots(as)
	str += "\n\n=====/// Snaps ///=====\n\n"
	for _,fs := range snaps {
		str += fmt.Sprintf("* %s\n", fs)
	}
*/
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK\n\n"+str))
}

// }}}

// {{{ AirspaceToSnapshots

func AirspaceToSnapshots(as *airspace.Airspace) []fdb.FlightSnapshot {
	ret := []fdb.FlightSnapshot{}

	for _,ad := range as.Aircraft {
		fs := fdb.FlightSnapshot{
			Trackpoint: fdb.TrackpointFromADSB(ad.Msg),
			Flight: fdb.BlankFlight(),
		}
		fs.Airframe = ad.Airframe
		fs.Identity.Schedule = ad.Schedule
		fs.Identity.IcaoId = string(ad.Msg.Icao24)
		fs.Identity.Callsign = ad.Msg.Callsign
		ret = append(ret, fs)
	}

	return ret
}

// }}}
// {{{ FlightSnapshotToAircraft

func FlightSnapshotToAircraft(snap *fdb.FlightSnapshot) *fr24.Aircraft {
	if snap == nil { return nil }

	return &fr24.Aircraft{
		Dist: snap.DistToReferenceKM,
		Dist3: snap.Dist3ToReferenceKM,
		BearingFromObserver: snap.BearingToReference,
		// Fr24Url string
		Id: snap.Flight.IdSpecString(),
		Id2: snap.Flight.IcaoId,
		Lat: snap.Trackpoint.Lat,
		Long: snap.Trackpoint.Long,
		Track: snap.Trackpoint.Heading,
		Altitude: snap.Trackpoint.Altitude,
		Speed: snap.Trackpoint.GroundSpeed,
		// Squawk string
		// Radar string
		EquipType: snap.Flight.EquipmentType,	
		Registration: snap.Flight.Registration,
		// Epoch float64
		Origin: snap.Flight.Origin,
		Destination: snap.Flight.Destination,
		FlightNumber: snap.Flight.BestFlightNumber(),	
		// Unknown float64
		VerticalSpeed: snap.Trackpoint.VerticalRate,
		Callsign: snap.Flight.Callsign,
		// Unknown2 float64
	}
}

// }}}

// {{{ FetchAirspace

func FetchAirspace(client *http.Client, bbox geo.LatlongBox) (*airspace.Airspace, string, error) {
	as := airspace.Airspace{}
	url := "http://fdb.serfr1.org/?json=1&"+bbox.ToCGIArgs("box")

	str := fmt.Sprintf("* FetchAirspace(%s)\n* %s\n", bbox, url)
	
	if resp,err := client.Get(url); err != nil {
		return nil, str, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, str, fmt.Errorf ("Bad status: %v", resp.Status)
	} else if err := json.NewDecoder(resp.Body).Decode(&as); err != nil {
		return nil, str, err
	}

	return &as, str, nil
}

// }}}
// {{{ FilterSnapshots

func FilterSnapshots(in []fdb.FlightSnapshot) []fdb.FlightSnapshot {
	out := []fdb.FlightSnapshot{}

	for _,snap := range in {
		if snap.Flight.BestFlightNumber() == "" { continue }
		if snap.Trackpoint.Altitude > 28000 { continue }
		if snap.Trackpoint.Altitude <   500 { continue }
		if snap.Dist3ToReferenceKM > 80 { continue }
		
		out = append(out, snap)
/*
		if a.Radar == "T-F5M" { continue }    // 5m delayed data; not what's overhead
		if a.FlightNumber == "" {continue}
		// if a.BestIdent() == "" { continue }  // No ID info; not much interesting to say
		if a.Altitude > 28000 { continue }    // Too high to be the problem
		if a.Altitude <   500 { continue }    // Too low to be the problem

		// Strip out little planes
		skip := false
		for _,e := range kBlacklistEquipmentTypes {
			if a.EquipType == e { skip = true }
		}
		if skip { continue }
*/
	}

	return out
}

// }}}
// {{{ SelectOverhead

func SelectOverhead(snaps []fdb.FlightSnapshot) (*fdb.FlightSnapshot, string) {
	if len(snaps) == 0 { return nil, "** empty list\n" }

	// closest plane has to be within 12 km to be 'overhead', and it has
	// to be 4km away from the next-closest
	if (snaps[0].Dist3ToReferenceKM < 12.0) {
		if (len(snaps) == 1) || (snaps[1].Dist3ToReferenceKM - snaps[0].Dist3ToReferenceKM) > 4.0 {
			return &snaps[0], "** selected 1st\n"

		} else {
			return nil, "** 2nd was too close to 1st\n"
		}

	} else {
		return nil, "** 1st was too far away\n"
	}

}

// }}}

// {{{ FindOverhead

func FindOverhead(client *http.Client, pos geo.Latlong, elev float64, grabAny bool) (*fr24.Aircraft, error, string) {
	str := fmt.Sprintf("*** FindOverhead for %s@%.0fft, at %s\n", pos, elev, date.NowInPdt())

	if pos.IsNil() {
		return nil, fmt.Errorf("flightid.FindOverhead needs a non-nil position"), str
	}
	
	bbox := pos.Box(64,64) // A box with sides ~40 miles, centered on the observer
	
	as, deb, err := FetchAirspace(client, bbox)
	str += deb
	if err != nil { return nil, err, str }

	snaps := AirspaceToSnapshots(as)
	for i,_ := range snaps {
		snaps[i].LocalizeTo(pos, elev)
	}
	sort.Sort(fdb.FlightSnapshotsByDist3(snaps))
	str += "** nearby list:-\n"+fdb.DebugFlightSnapshotList(snaps)

	filtered := FilterSnapshots(snaps)
	str += "** filtered:-\n"+fdb.DebugFlightSnapshotList(filtered)
	if len(filtered) == 0 { return nil, nil, str }

	overheadSnap,debug := SelectOverhead(filtered)
	if overheadSnap == nil && grabAny {
		overheadSnap = &filtered[0]
		debug += "** grabbing first anyway\n"
	}
	str += debug

	return FlightSnapshotToAircraft(overheadSnap), nil, str
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
