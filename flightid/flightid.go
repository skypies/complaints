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

	fdb "github.com/skypies/flightdb"
	"github.com/skypies/flightdb/fr24"
	"github.com/skypies/geo"
	"github.com/skypies/pi/airspace"
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
		names := SelectorNames
		names = append([]string{"conservative"}, names...)
		for _,name := range names {
			algo := NewSelector(name)
			oh,debstr := IdentifyOverhead(as, pos, 0.0, algo)
			str += fmt.Sprintf("--{ IdentifyOverhead, algo: %s }--\n -{ OH: %s }-\n\n%s\n",
				algo, oh, debstr)
		}
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
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
			BearingFromObserver: pos.BearingTowards(tp.Latlong),
			//Id: "someid", // This is set via an IdSpec string below
			Id2: icaoid,
			// Squawk string
			EquipType: ad.Airframe.EquipmentType,	
			Registration: ad.Airframe.Registration,
			////Epoch: float64(tp.TimestampUTC.Unix()),
			Origin: ad.Schedule.Origin,
			Destination: ad.Schedule.Destination,
			FlightNumber: ad.Schedule.IataFlight(),
			// Unknown float64
			Callsign: ad.Msg.Callsign,
			// Unknown2 float64
		}

		a.FromTrackpoint(tp) // Populate basic fields from the trackpoint
		
		// Even though we may be parsing an fr24 airspace, generate a (perhaps invalid) skypi idspec
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
	header := "3Dist  2Dist  Brng   Hdng    Alt      Speed VertSpd  Equp Flight   Orig:Dest IcaoID "+
		"Callsign Source  Age\n"

	lines := []string{}

	for _,a := range list {
		line := fmt.Sprintf(
			"%4.1fKM %4.1fKM %3.0fdeg %3.0fdeg %6.0fft %4.0fkt %5.0ffpm "+
			"%4.4s %-8.8s %4.4s:%-4.4s %6.6s %-8.8s %-7.7s "+
			"%2.0fs\n",
			a.Dist3, a.Dist, a.BearingFromObserver, a.Track, a.Altitude, a.Speed, a.VerticalSpeed,
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
// {{{ TimeSyncAircraft

// datapoints have ages from 3 to 30s. We sync them to a target age, doing some linear extrapolation
// to adjust the position and altitude. The idea is that they're a more accurate set of data to
// pick a culprit from.

func TimeSyncAircraft(in []Aircraft, pos geo.Latlong, elev float64, targetAge time.Duration) []Aircraft {
	out := []Aircraft{}
	
	for i,_ := range in {
		new := in[i] // copy it over, then move the new one

		actualAge := time.Since(time.Unix(int64(new.Epoch),0))
		timeToWind := actualAge - targetAge

		newTP := new.Trackpoint().RepositionByTime(timeToWind)
		new.FromTrackpoint(newTP)

		new.Dist = pos.DistKM(new.Latlong())
		new.Dist3 = pos.Dist3(new.Latlong(), new.Altitude-elev)
		new.BearingFromObserver = pos.BearingTowards(new.Latlong())

		out = append(out, new)
	}

	// Resort the array
	sort.Sort(AircraftByDist3(out))

	return out
}

// }}}
// {{{ IdentifyOverhead

func IdentifyOverhead(as *airspace.Airspace, pos geo.Latlong, elev float64, algo Selector) (*Aircraft, string) {
	if as == nil { return (*Aircraft)(nil), "** airspace was nil\n" }	

	nearby := AirspaceToLocalizedAircraft(as, pos, elev)
	filtered := FilterAircraft(nearby)

	if true {
		targetAge := 6 * time.Second
		filtered = TimeSyncAircraft(filtered, pos, elev, targetAge)
	}

	ret, outcome := (*Aircraft)(nil), "nothing found in the sky"
	if len(filtered) > 0 {
		ret,outcome = algo.Identify(pos,elev,filtered)
	}

	str := fmt.Sprintf("**** identification method: %s\n**** outcome: %s\n\n", algo, outcome)
	str += fmt.Sprintf("** Processed [%d] **\n%s\n", len(filtered), AircraftToString(filtered))
	str += fmt.Sprintf("** Raw [%d] **\n%s\n", len(nearby), AircraftToString(nearby))

	return ret,str
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
