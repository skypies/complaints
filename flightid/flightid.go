package flightid

import(
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	fdb "github.com/skypies/flightdb2"
	"github.com/skypies/flightdb2/fr24"
	"github.com/skypies/geo"
	"github.com/skypies/pi/airspace"
)

type Algorithm int
const(
	AlgoConservativeNoCongestion Algorithm = iota
	AlgoGrabClosest
)

func init() {
	http.HandleFunc("/cdb/airspace", airspaceHandler)
}

// {{{ airspaceHandler

func airspaceHandler(w http.ResponseWriter, r *http.Request) {
	client := urlfetch.Client(appengine.NewContext(r))
	pos := geo.Latlong{37.060312,-121.990814}
	str := ""

	if as,err := fr24.FetchAirspace(client, pos.Box(100,100)); err != nil {
		http.Error(w, fmt.Sprintf("FetchAirspace: %v", err), http.StatusInternalServerError)
		return
	} else {		
		oh,debstr := IdentifyOverhead(as, pos, 0.0, AlgoConservativeNoCongestion)
		str += fmt.Sprintf("--{ IdentifyOverhead }--\n -{ OH: %s }-\n\n%s\n", oh, debstr)
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK\n\n"+str))
}

// }}}

// {{{ AirspaceToLocalizedAircraft

func AirspaceToLocalizedAircraft(as *airspace.Airspace, pos geo.Latlong, elev float64) []Aircraft {
	ret := []Aircraft{}

	if as == nil { return ret }
	
	for _,ad := range as.Aircraft {
		tp := fdb.TrackpointFromADSB(ad.Msg)
		altitudeDelta := tp.Altitude - elev

		icaoid := string(ad.Msg.Icao24)
		icaoid = strings.TrimPrefix(icaoid, "EE") // fr24 airspaces use this prefix
		icaoid = strings.TrimPrefix(icaoid, "FF") // fa airspaces use this prefix
		
		a := Aircraft{
			Dist: pos.DistKM(tp.Latlong),
			Dist3: pos.Dist3(tp.Latlong, altitudeDelta),
			BearingFromObserver: tp.Latlong.BearingTowards(pos),
			//Id: "someid", // This is set via an IdSpec string below
			Id2: icaoid,
			Lat: tp.Lat,
			Long: tp.Long,
			Track: tp.Heading,
			Altitude: tp.Altitude,
			Speed: tp.GroundSpeed,
			// Squawk string
			Radar: tp.ReceiverName,
			EquipType: ad.Airframe.EquipmentType,	
			Registration: ad.Airframe.Registration,
			Epoch: float64(tp.TimestampUTC.Unix()),
			Origin: ad.Schedule.Origin,
			Destination: ad.Schedule.Destination,
			FlightNumber: ad.Schedule.IataFlight(),
			// Unknown float64
			VerticalSpeed: tp.VerticalRate,
			Callsign: ad.Msg.Callsign, //"CAL123", //snap.Flight.Callsign,
			// Unknown2 float64
		}

		// Even though we may be parsing an fr24 airspace, generate a skypi idspec (which may break)
		if icaoid != "" {
			a.Id = fdb.IdSpec{ IcaoId:icaoid, Time:tp.TimestampUTC }.String()
		}

		// Hack for Surf Air; promote their callsigns into flightnumbers.
		// The goal is to allow them to get past the filter, and be printable etc.
		// There is probably a much better place for this kind of munging. But it needs to happen
		//  regardless of the source of the airspace, so it can't be pushed upstream. (...?)
		if a.FlightNumber == "" && regexp.MustCompile("^URF\\d+$").MatchString(a.Callsign) {
			a.FlightNumber = a.Callsign
		}
		
		ret = append(ret, a)
	}

	sort.Sort(AircraftByDist3(ret))
	
	return ret
}

// }}}
// {{{ AircraftToString

func AircraftToString(list []Aircraft) string {
	header := "3Dist  2Dist  Brng   Hdng    Alt      Speed Equp Flight   Orig:Dest IcaoID "+
		"Callsign Source  Age\n"

	lines := []string{}

	for _,a := range list {
		line := fmt.Sprintf(
			"%4.1fKM %4.1fKM %3.0fdeg %3.0fdeg %6.0fft %4.0fkt "+
			"%4.4s %-8.8s %4.4s:%-4.4s %6.6s %-8.8s %-7.7s "+
			"%2.0fs\n",
			a.Dist3, a.Dist, a.BearingFromObserver, a.Track, a.Altitude, a.Speed,
			a.EquipType, a.FlightNumber,
			a.Origin, a.Destination, a.Id2, a.Callsign, a.Radar,
			time.Since(time.Unix(int64(a.Epoch),0)).Seconds())

		lines = append(lines, line)
	}

	//sort.Strings(lines)

	return header + strings.Join(lines, "")
}

// }}}
// {{{ FilterAircraft

func FilterAircraft(in []Aircraft) []Aircraft {
	out := []Aircraft{}

	for _,a := range in {
		age := time.Since(time.Unix(int64(a.Epoch),0))

		if age > 30 * time.Second { continue } // Data too old to use
		if a.FlightNumber == "" { continue }   // Drop unscheduled traffic; poor way of skipping GA
		if a.Altitude > 28000 { continue }     // Too high to be the problem
		if a.Altitude <   750 { continue }     // Too low to be the problem

		out = append(out, a)
	}

	return out
}

// }}}
// {{{ IdentifyOverhead

func IdentifyOverhead(as *airspace.Airspace, pos geo.Latlong, elev float64, algo Algorithm) (*Aircraft, string) {
	if as == nil { return nil, "** airspace was nil" }
	
	str := "" // fmt.Sprintf("** Airspace **\n%s\n\n", as)

	nearby := AirspaceToLocalizedAircraft(as, pos, elev)
	filtered := FilterAircraft(nearby)
	index,outcome := SelectViaAlgorithm(algo,pos,elev,filtered)

	str += fmt.Sprintf("** Unfiltered [%d] **\n%s\n", len(nearby), AircraftToString(nearby))
	str += fmt.Sprintf("** Filtered [%d] **\n%s\n", len(filtered), AircraftToString(filtered))
	str += outcome

	if index < 0 {
		return nil,str
	} else {
		return &filtered[index], str
	}
}

// }}}

// {{{ SelectViaAlgorithm

func SelectViaAlgorithm(algo Algorithm, pos geo.Latlong, elev float64, aircraft []Aircraft) (int,string) {
	if len(aircraft) == 0 {
		return -1, "** no filtered aircraft in range\n"
	}
	
	switch algo {
	case AlgoConservativeNoCongestion: return SelectViaConservativeHeuristic(aircraft)
	case AlgoGrabClosest: return 0, "** grabbed closest\n"
	default: return -1, fmt.Sprintf("** algorithm '%v' not known\n", algo)
	}
}

// }}}
// {{{ SelectViaConservativeHeuristic

// The original, "no congestion allowed" heuristic ...
//
// closest plane has to be within 12 km to be 'overhead', and it has
// to be 4km away from the next-closest

func SelectViaConservativeHeuristic(in []Aircraft) (int, string) {
	if (in[0].Dist3 < 12.0) {
		if (len(in) == 1) || (in[1].Dist3 - in[0].Dist3) > 4.0 {
			return 0, "** selected 1st\n"

		} else {
			return -1, "** 2nd was too close to 1st\n"
		}

	} else {
		return -1, "** 1st was too far away\n"
	}

}

// }}}

// Prob 1: URF on skypi. fr24 rewrites callsign into 'schedule number'; but the raw MLAT data
//  has the registration as the callsign. So fr24 know how to map registration into 'schedule'.
// Even more annoying, fr24 doesn't have IcaoID at this time, because it's on near-time FAA.
//  fr24:  ["bc96f5f","",36.8055,-122.1902,162,15700,271,"0000","T-F5M","PC12","",1480729635,"SQL","SBA","",0,1088,"URF133",0]
//  skypi: http://fdb.serfr1.org/fdb/tracks?idspec=AC06C3@1480729827
// BUT BUT BUT ... the schedcache may save us, as it maps icaoids to flightnumbers; and with
//  the fr24/URF hack, we'll now have those flightnumbers. Maybe !

// The datatypes in this file can be confusing. The input:
//
//   pi/airspace.Airspace         : the input (from skypi, or fr24, or anything else)
//     pi/airspace.AircraftData   : an ADSB message-based model of a flight at a point in time
//       fdb/schedule.Schedule    : flightnumber, origin.dest, etc. If we have it.
//       fdb/airframe.Airframe    : equipment type, registration, keyed from IcaoId. If we have it.
//
// And the output, only used within the complaints system:
//
//   complaints/flightid.Aircraft : a flattened subset of the above, localized to the user

/* New algorithm ...

Inputs:
 list of flights, each with a position, groundspeed, altitude, and timestamp.
 user, also with a position, elevation, and timestamp (of button press)

0. [Optional fudge]: apply a handicap delay to the user (assume they're late pressing the button)

1. Forward-extrapolate aircraft's position, based on (user.tstamp - flight.tstamp) * flight.velocity
This is approximate; assumes level flight, and no acceleration.

2. Compute the time delay for sound propagation, based on 3D distance
This is approximate; speed of sound will vary with atmosphere.

3. Back-extrapolate the position of the aircraft at the time the button was pressed.

4. Compute distance between aircraft and user.

5. Sort aircraft by this distance.

A jet at 10,000', and 4KM lateral, is ~5KM away.
A cessna at 2,000', and 4KM lateral, is <5KM away, and would be closest.

 */


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
