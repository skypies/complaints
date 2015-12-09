
package fr24
// http://blog.cykey.ca/post/88174516880/analyzing-flightradar24s-internal-api-structure
// https://github.com/danmir/pyFlightRadar/blob/master/get_planes.py

// {{{ Global setup

import (
	"encoding/json"
	"io/ioutil"
	"fmt"
	"net/http"
	"regexp"
	"sort"

	"github.com/skypies/date"
	"github.com/skypies/geo"
	//"github.com/skypies/geo/sfo"

	"github.com/skypies/complaints/config"
)

var(
	// kBalancerUrl = "http://www.flightradar24.com/balance.json"
	kBalancerUrl = config.Get("fr24.kBalancerUrl")

	kListUrlPath = config.Get("fr24.kListUrlPath")
	kLiveFlightDetailsUrlPath = config.Get("fr24.kLiveFlightDetailsUrlPath")
	kFlightDetailsUrlStem = config.Get("fr24.kFlightDetailsUrlStem")
	
	kPlaybackUrl2 = config.Get("fr24.kPlaybackUrl2")
	kPlaybackUrl = config.Get("fr24.kPlaybackUrl")
	
	kBlacklistEquipmentTypes = []string{
		"SR20",
	}
)

type Fr24 struct {
	Client *http.Client
	host string  // As reported by the balanceUrl
}

// }}}

// {{{ type Aircraft

// ["70795fd", "A4243B", 36.6846, -121.8509, 330,
//   19128, 0, "2037", "T-MLAT2", "B733",
//   "N366SW", 1438819247, "LAX", "SFO", "WN482",
//   0, -1920, "SWA482", 0]
type Aircraft struct {
	//`datastore:"-"` // for all these ??
	Dist                float64  // in KM
	Dist3               float64  // in KM (3D dist, taking altitude into account)
	BearingFromObserver float64  // bearing from the house
	Fr24Url             string   // Flightradar's playback view

	
	Id string            // Flightradar's ID for this instance of this flight
	Id2 string           // Better known as ModeS
	Lat float64
	Long float64
	Track float64

	Altitude float64
	Speed float64
	Squawk string
	Radar string
	EquipType string
	
	Registration string
	Epoch float64
	Origin string
	Destination string
	FlightNumber string
	
	Unknown float64
	VerticalSpeed float64
	Callsign string
	Unknown2 float64
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
	if a.FlightNumber != "" {
		if s := regexp.MustCompile("^(..)(\\d+)$").ReplaceAllString(a.FlightNumber, "$1"); s != "" {
			return s
		} else {
			return a.FlightNumber // Shouldn't happen
		}
	}
	return ""
}

// }}}
// {{{ type AircraftLiveDetails

// {{{ Sample data

/*
{
  "aircraft": "Boeing 737-7H4",
  "airline": "Southwest Airlines",
  "airline_url": "http://www.flightradar24.com/data/airplanes/swa-swa",
  "arr_schd": 1440084000,
  "arrival": 1440083976,
  "copyright": "Mark Pollio",
  "copyright_large": "Jean-Luc Poliquin",
  "dep_schd": 1440080100,
  "departure": 1440080717,
* "eta": 1440083976,
  "first_timestamp": 0,
  "flight": "WN2812",
  "from_city": "Burbank, Burbank Bob Hope Airport",
* "from_iata": "BUR",
  "from_pos": [ 34.200661, -118.358002 ],
  "from_tz_code": "PDT",
  "from_tz_name": "Pacific Daylight Time",
  "from_tz_offset": "-7.00",
  "image": "http://img.planespotters.net/photo/330000/thumbnail/PlanespottersNet_330615.jpg?v=0",
  "image_large": "http://img.planespotters.net/...",
  "imagelink": "http://external.flightradar24.com/...",
  "imagelink_large": "http://external.flightradar24.com/...",
  "imagesource": "Planespotters.net",
  "q": "72d84f7",
  "snapshot_id": null,
  "status": "airborne",
  "to_city": "San Jose, San Jose International Airport",
* "to_iata": "SJC"
  "to_pos": [ 37.362659, -121.929001 ],
  "to_tz_code": "PDT",
  "to_tz_name": "Pacific Daylight Time",
  "to_tz_offset": "-7.00",
    
  "trail": [
      37.379,     -121.944,    5,
      37.3743,    -121.943,    5,
      ....
  ],
}
*/

// }}}

type AircraftLiveDetails struct {
	ETAUnixEpoch  int     `json:"eta"`
	FromIATA      string  `json:"from_iata"`
	ToIata        string  `json:"to_iata"`

	RawJson       string
}

// }}}
// {{{ type AircraftPlayback

// {{{ Sample data

/*

Top level structs in response:
 result.response.data.flight.{airline,identification,aircraft,airport,track}
 result.response.timestamp
 result.request

{
  "result": {
    "response": {
      "data": {
        "flight": {

          "airline": {
            "name": "Southwest Airlines",
            "code": {
              "icao": "SWA",
              "iata": "WN"
            }
          },

          "identification": {
            "hex": "72d84f7",
            "id": 120423671,
            "number": {
              "default": "WN2812"
            },
            "callsign": null
          },
            
          "aircraft": {
            "identification": {
              "modes": "A53F20",
              "registration": "N437WN"
            },
            "model": {
              "text": "Boeing 737-7H4",
              "code": "B737"
            }
          },

          "airport": {
            "destination": {
              "name": "San Jose International Airport",
              "code": {
                "icao": "KSJC",
                "iata": "SJC"
              },
              "timezone": {
                "offset": -25200,
                "abbr": "PDT",
                "abbrName": "Pacific Daylight Time",
                "name": "America/Los_Angeles",
                "isDst": true
              },
              "position": {
                "country": {
                  "name": "United States",
                  "code": "US"
                },
                "latitude": 37.362659,
                "region": {
                  "city": "San Jose"
                },
                "longitude": -121.929001
              }
            },
            "origin": {
              "name": "Burbank Bob Hope Airport",
              "code": {
                "icao": "KBUR",
                "iata": "BUR"
              },
              "timezone": {
                "offset": -25200,
                "abbr": "PDT",
                "abbrName": "Pacific Daylight Time",
                "name": "America/Los_Angeles",
                "isDst": true
              },
              "position": {
                "country": {
                  "name": "United States",
                  "code": "US"
                },
                "latitude": 34.200661,
                "region": {
                  "city": "Burbank"
                },
                "longitude": -118.358002
              }
            }
          },

          "track": [
            {
              "heading": 255,
              "latitude": 34.1408,
              "speed": {
                "kts": 140,
                "mph": 161.1,
                "kmh": 259.3
              },
              "squawk": "0",
              "altitude": {
                "feet": 4653,
                "meters": 1418
              },
              "longitude": -118.421,
              "timestamp": 1440080996
            },
            ...

          ]
        }
      },
      "timestamp": 1440084175
    },
    "request": {
      "format": "json",
      "flightId": "72D84F7",
      "callback": null
    }
  },
  "_api": {
    "copyright": "Copyright (c) 2012-2015 Flightradar24 AB. All rights reserved.",
    "legalNotice": "The contents of this file and all derived data are the property of Flightradar24 AB for use exclusively by its products and applications. Using, modifying or redistributing the data without the prior written permission of Flightradar24 AB is not allowed and may result in prosecutions.",
    "version": "1.0.12"
  }
}
 */

// }}}

type AircraftPlayback struct {
}

// }}}

// {{{ aircraft.PlaybackUrl

// When you lookup a specific flight instance via the Playback page, it issues this URL
// to fetch a full set of flight details and track history. Notably, it has timestamps
// in the trail. This URL is only available for ~6 days after the flight has landed. It
// is available before the flight has landed.

// http://mobile.api.fr24.com/common/v1/flight-playback.json?flightId=729a70e

// E.g. http://www.flightradar24.com/data/flights/WN2182/#72D84F7
// E.g. http://www.flightradar24.com/data/flights/UA1570/#741d9c7

func (a *Aircraft) PlaybackUrl() string {
	if a.FlightNumber == "" {
		return fmt.Sprintf("http://www.flightradar24.com/reg/%s", a.Registration)
	}
	return fmt.Sprintf("%s%s/#%s", kFlightDetailsUrlStem, a.FlightNumber, a.Id)
}

// }}}
// {{{ aircraft.LiveDetailsUrl

// When you click a flight on the map, it calls this URL to populate the flight details box,
// and get some trail data. It is incomplete in a few ways (trail has no timestamps, flight
// might not have landed !).

// E.g. http://krk.fr24.com/_external/planedata_json.1.3.php?f=72d84f7

func (a Aircraft) LiveDetailsUrl(host string) string {
	return fmt.Sprintf("http://%s%s?f=%s", host, kLiveFlightDetailsUrlPath, a.Id)
}

// }}}
// {{{ aircraft.String

func (a *Aircraft) String() string {
	return fmt.Sprintf("%s[%s:%s-%s]", a.Id, a.FlightNumber, a.Origin, a.Destination)
}

// }}}

// {{{ fr24.Url2resp

func (fr *Fr24) Url2resp(url string) (resp *http.Response, err error) {
	if resp,err = fr.Client.Get(url); err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf ("Bad status: %v", resp.Status)
		return
	}

	return
}

// }}}
// {{{ fr24.Url2body

func (fr *Fr24) Url2body(url string) (body []byte, err error) {
	if resp,err := fr.Url2resp(url); err != nil {
		return nil, err
	} else {
		defer resp.Body.Close()
		return ioutil.ReadAll(resp.Body)
	}
}

// }}}
// {{{ fr24.Url2jsonMap

func (fr *Fr24) Url2jsonMap(url string) (jsonMap map[string]interface{}, err error) {
	resp,err2 := fr.Url2resp(url)
	if err2 != nil { err = err2; return }
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&jsonMap)
	return
}

// }}}
// {{{ fr24.getHostname

// Ask the load balancer which host to use
// http://blog.cykey.ca/post/88174516880/analyzing-flightradar24s-internal-api-structure
func (fr *Fr24) getHostname() error {
	jsonMap,err := fr.Url2jsonMap(kBalancerUrl)
	if err != nil {
		return err
	}

	min := 99999.0
	fr.host = ""
	for k,v := range jsonMap {
		score := v.(float64)
		if (score < min) {
			fr.host,min = k,score
		}
	}

	return nil
}

// }}}
// {{{ fr24.EnsureHostname

// {"krk.data.fr24.com":250,"bma.data.fr24.com":250,"arn.data.fr24.com":250,"lhr.data.fr24.com":250}
func (fr *Fr24) EnsureHostname() error {
	if fr.host == "" {
		fr.host = "krk.data.fr24.com"
		return nil
		/*
		if err := fr.getHostname(); err != nil {
			return err
		}
*/
	}
	return nil
}

// }}}

// {{{ fr24.ParseListBbox

// Used temporarily for dwelltime stuff
func (fr *Fr24) ParseListBbox(jsonMap map[string]interface{}, aircraft *[]Aircraft) {
	// Unpack the aircraft summary object
	for _,v := range jsonMap["aircraft"].([]interface{}) {
		a := Aircraft{
			Id: v.([]interface{})[0].(string),
			Id2: v.([]interface{})[1].(string),
			Lat: v.([]interface{})[2].(float64),
			Long: v.([]interface{})[3].(float64),
			Track: v.([]interface{})[4].(float64),
			Altitude: v.([]interface{})[5].(float64),
			Speed: v.([]interface{})[6].(float64),
			Squawk: v.([]interface{})[7].(string),
			Radar: v.([]interface{})[8].(string),
			EquipType: v.([]interface{})[9].(string),
			Registration: v.([]interface{})[10].(string),
			Epoch: v.([]interface{})[11].(float64),
			Origin: v.([]interface{})[12].(string),
			Destination: v.([]interface{})[13].(string),
			FlightNumber: v.([]interface{})[14].(string),
			Unknown: v.([]interface{})[15].(float64),
			VerticalSpeed: v.([]interface{})[16].(float64),
			Callsign: v.([]interface{})[17].(string),
			Unknown2: v.([]interface{})[18].(float64),
		}
		*aircraft = append(*aircraft,a)
	}

	return
}

// }}}
// {{{ fr24.ListBbox

func (fr *Fr24) ListBbox(sw_lat,sw_long,ne_lat,ne_long float64, aircraft *[]Aircraft) error {
	if err := fr.EnsureHostname(); err != nil {	return err }

	bounds := fmt.Sprintf("%.3f,%.3f,%.3f,%.3f", ne_lat, sw_lat, sw_long, ne_long)
	url := fmt.Sprintf("http://%s%s?array=1&bounds=%s", fr.host, kListUrlPath, bounds)

	jsonMap,err := fr.Url2jsonMap(url)
	if err != nil { return err }

	// Unpack the aircraft summary object
	for _,v := range jsonMap["aircraft"].([]interface{}) {
		a := Aircraft{
			Id: v.([]interface{})[0].(string),
			Id2: v.([]interface{})[1].(string),
			Lat: v.([]interface{})[2].(float64),
			Long: v.([]interface{})[3].(float64),
			Track: v.([]interface{})[4].(float64),
			Altitude: v.([]interface{})[5].(float64),
			Speed: v.([]interface{})[6].(float64),
			Squawk: v.([]interface{})[7].(string),
			Radar: v.([]interface{})[8].(string),
			EquipType: v.([]interface{})[9].(string),
			Registration: v.([]interface{})[10].(string),
			Epoch: v.([]interface{})[11].(float64),
			Origin: v.([]interface{})[12].(string),
			Destination: v.([]interface{})[13].(string),
			FlightNumber: v.([]interface{})[14].(string),
			Unknown: v.([]interface{})[15].(float64),
			VerticalSpeed: v.([]interface{})[16].(float64),
			Callsign: v.([]interface{})[17].(string),
			Unknown2: v.([]interface{})[18].(float64),
		}
		*aircraft = append(*aircraft,a)
	}

	return nil
}

// }}}

// {{{ DebugFlightList

func DebugFlightList(aircraft []Aircraft) string {
	debug := "3Dist  2Dist  Brng   Hdng    Alt      Speed Equp Flight   Latlong\n"

	for _,a := range aircraft {
		debug += fmt.Sprintf(
			"%4.1fKM %4.1fKM %3.0fdeg %3.0fdeg %6.0fft %4.0fkt %s %-8.8s (% 8.4f,%8.4f)\n",
			a.Dist3, a.Dist, a.BearingFromObserver, a.Track, a.Altitude, a.Speed,
			a.EquipType, a.BestIdent(), a.Lat, a.Long)
	}

	return debug
}

// }}}

// {{{ fr24.filterAircraft

func filterAircraft(in []Aircraft) (out []Aircraft) {
	for _,a := range in {
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
		
		out = append(out, a)
	}
	return
}

// }}}
// {{{ fr24.FindOverhead

type byDist []Aircraft
func (s byDist) Len() int      { return len(s) }
func (s byDist) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byDist) Less(i, j int) bool { return s[i].Dist < s[j].Dist }

type byDist3 []Aircraft
func (s byDist3) Len() int      { return len(s) }
func (s byDist3) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byDist3) Less(i, j int) bool { return s[i].Dist3 < s[j].Dist3 }


func (fr *Fr24) FindOverhead(observerPos geo.Latlong, overhead *Aircraft, grabAnything bool) (debug string, err error) {
	
	debug = fmt.Sprintf("*** FindOverhead for %s, at %s\n", observerPos,
		date.NowInPdt())
	
	// Create a bounding box that's ~40m square, centred on the input lat,long
	// This is a grievous fudge http://www.movable-type.co.uk/scripts/latlong.html
	lat_20miles := 0.3
	long_20miles := 0.35
	nearby := []Aircraft{}
	if err = fr.ListBbox(observerPos.Lat-lat_20miles, observerPos.Long-long_20miles,
		observerPos.Lat+lat_20miles, observerPos.Long+long_20miles, &nearby); err != nil {
			debug += fmt.Sprintf("Lookup error: %s\n", err)
			return
	}

	for i,a := range nearby {
		aircraftPos := geo.Latlong{a.Lat,a.Long}
		nearby[i].Dist = observerPos.Dist(aircraftPos)
		nearby[i].Dist3 = observerPos.Dist3(aircraftPos, a.Altitude)
		nearby[i].BearingFromObserver = observerPos.BearingTowards(aircraftPos)
	}
	sort.Sort(byDist3(nearby))
	debug += "** nearby list:-\n"+DebugFlightList(nearby)

	filtered := filterAircraft(nearby)
	if len(filtered) == 0 {
		debug += "** all empty after filtering\n"
		return
	}

	debug += "** filtered:-\n"+DebugFlightList(filtered)

	if grabAnything {
		*overhead = filtered[0]
		debug += "** grabbed 1st\n"
	} else {
		// closest plane has to be within 12 km to be 'overhead', and it has
		// to be 4km away from the next-closest
		if (filtered[0].Dist3 < 12.0) {
			if (len(filtered) == 1) || (filtered[1].Dist3 - filtered[0].Dist3) > 4.0 {
				*overhead = filtered[0]
				debug += "** selected 1st\n"
			} else {
				debug += "** 2nd was too close to 1st\n"
			}
		} else {
			debug += "** 1st was too far away\n"
		}
	}
	
	return
}

// }}}

// Helpers for flightdbfr24
func (fr Fr24) GetPlaybackUrl(id string) string {
	return fmt.Sprintf("%s?flightId=%s", kPlaybackUrl2, id)
}
func (fr Fr24) GetListUrl(bounds string) string {
	return fmt.Sprintf("http://%s%s?array=1&bounds=%s", fr.host, kListUrlPath, bounds)
}
func (fr Fr24) GetLiveDetailsUrl(id string) string {
	return fmt.Sprintf("http://lhr.data.fr24.com/_external/planedata_json.1.3.php?f=%s", id)
}

// {{{ Notes

/*

---- Snapshotter -----
Runs every N minutes, via cron
Should be singleton (does this really matter though, it's all kinda idempotent)

Loads up SnapshotterState, a buffer of Fr24 hex IDs that were in flight last time
Does a lookup call against fr24
Computes diffs:
 * new flights we've not seen before
 * flights we've not seen for an hour
 * updates "last seen" on all the others still in the frame
Update SnapshotterState

Foreach new flight, add a task entry:
 NotBefore: inbound flights, ETA plus 10 minutes ? Outbound .... just 30 minutes ?
 Action: EnsureWeHaveTrack (if not in the DB, grab the instance)
 

Facts:
 NorCal Metroplex is SFO,SJC,OAK.
 Generous bounding box: [36.575219, -123.050353, 38.017199, -121.542479] (134km x 160km)
   http://krk.fr24.com//zones/fcgi/feed.json?array=1&bounds=ne_lat,sw_lat,sw_long,ne_long
   http://krk.fr24.com//zones/fcgi/feed.json?array=1&bounds=38.017199,36.575219,-123.050353,-121.542479
 This matched 109 flights (16:41, Thu). The flightradar GUI reported ~106 for a similar area.

 */

/*

Flight details (realtime?): http://krk.fr24.com/_external/planedata_json.1.3.php?f=72ad32d"
 - 249 datapoints, [lat,long,speed,alt] - no timestamps

Fullpage playback: http://www.flightradar24.com/data/flights/WN2056/#729a70e
 - 147 datapoints ?? has timestamp. Marry up.

Playback JSON data:  http://mobile.api.fr24.com/common/v1/flight-playback.json?flightId=729a70e


// Web: Base page / flight details URL - uses long flightnumber
http://www.flightradar24.com/SWA2812/72d84f7

// Web: History URL - uses short flight number
http://www.flightradar24.com/data/flights/WN2812/#72d84f7


// Json: Flight details
http://krk.fr24.com/_external/planedata_json.1.3.php?f=72d84f7
// lines [43,409) == 366 lines; 3 lines per point means 122 points. Yay!
// Sample point:       35.3288,    -119.3,      3599.9,


// Json: Playback data
http://mobile.api.fr24.com/common/v1/flight-playback.json?flightId=72d84f7
// 122 data points (grep -c heading)
// Sample point:
            {
              "heading": 316,
              "latitude": 35.3288,
              "longitude": -119.3,
              "timestamp": 1440081778
              "altitude": {
                "feet": 35999,
                "meters": 10972
              },
              "speed": {
                "kts": 286,
                "mph": 329.1,
                "kmh": 529.7
              },
              "squawk": "1312",
            },

// So: the realtime feed contains three fields - {lat, long, altitude.feet/10}

WN2056/#729a70e


-=-=-=-=-=-=-=-=-=-

Plan For DB

A. Precompute timestamps when flight was "within bbox of interest" (all of SF metroplex)
B. Seach for flights who have datapoints within bbox, and within timerange

(B) sounds expensive.

 */

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
