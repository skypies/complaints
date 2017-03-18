package flightid

import(
	"fmt"
	"math"
	"math/rand"
	"sort"
	"github.com/skypies/geo"
)

// Randomness in public: Fri Mar 17, 18:00, until Sat Mar 18, 11:00

// Selector is a role for things (algorithms) that can select a problem aircraft from an airspace
type Selector interface {
	String() string

	// Pick out the noise-making flight from the array, and a oneline explanation. If none
	// was identified, returns nil (but the string will explain why).
	// The aircraft slice is initially sorted by Dist3.
	Identify(pos geo.Latlong, elev float64, aircraft []Aircraft) (*Aircraft, string)
}

// SelectorNames is the list of selectors we want users to be able to pick from
var SelectorNames = []string{"conservative","cone"}

func NewSelector(name string) Selector {
	switch name {
	case "random": return AlgoRandom{}
	case "conservative": return AlgoConservativeNoCongestion{}
	case "cone": return AlgoLowestInCone{}
	default: return AlgoConservativeNoCongestion{}
	}
}
func ListSelectors() [][]string {
	ret := [][]string{}
	for _,name := range SelectorNames {
		ret = append(ret, []string{name, NewSelector(name).String()})
	}
	return ret
}

type AlgoRandom struct{}
func (a AlgoRandom)String() string { return "Picks at random" }
func (a AlgoRandom)Identify(pos geo.Latlong, elev float64, in []Aircraft) (*Aircraft,string) {
	if len(in) == 0 {
		return nil, "list was empty"
	}
	i := rand.Intn(len(in))
	return &in[i], fmt.Sprintf("random (grabbed aircraft %d from list of %d)", i+1, len(in))
}

// The original, "no congestion allowed" heuristic ...
type AlgoConservativeNoCongestion struct{}
func (a AlgoConservativeNoCongestion)String() string { return "Conservative, gives up on congestion" } 
func (a AlgoConservativeNoCongestion)Identify(pos geo.Latlong, elev float64, in []Aircraft) (*Aircraft,string) {
	if (in[0].Dist3 >= 12.0) {
		return nil, "not picked; 1st closest was too far away (>12KM)"
	} else if (len(in) == 1) || (in[1].Dist3 - in[0].Dist3) > 4.0 {
		return &in[0], "selected 1st closest"
	} else {
		return nil, "not picked; 2nd closest was too close to 1st (<4KM)"
	}
}


type AlgoLowestInCone struct{}
func (a AlgoLowestInCone)String() string {
	return "Picks lowest inside a 60deg cone [EXPERIMENTAL]"
}

func (a AlgoLowestInCone)Identify(pos geo.Latlong, elev float64, in []Aircraft) (*Aircraft,string) {
	if len(in) == 0 {
		return nil, "nothing in list"
	} else if (in[0].Dist3 >= 12.0) {
		return nil, "not picked; 1st closest was too far away (>12KM)"
	} else if (len(in) == 1) || (in[1].Dist3 - in[0].Dist3) > 4.0 {
		return &in[0], "selected 1st closest"
	}

	// Build new list of just those flights which are within the cone (i.e. angle <60deg)
	enconed := []Aircraft{}
	for _,a := range in {
		// angle between a vertical line from pos, and the line from pos to the aircraft.
		horizDistKM := a.Dist
		vertDistKM := (a.Altitude - elev) / geo.KFeetPerKM
		if angle := math.Atan2(horizDistKM,vertDistKM) * (180.0 / math.Pi); angle <= 60 {
			enconed = append(enconed, a)
		}
	}

	if len(enconed) == 0 {
		return nil, "not picked; nothing found inside 60deg cone."
	}

	sort.Sort(AircraftByAltitude(enconed))
	return &enconed[0], "picked lowest in cone."
}
