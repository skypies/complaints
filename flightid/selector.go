package flightid

import(
	"fmt"
	"math"
	"math/rand"
	"github.com/skypies/geo"
)

// Selector is a role for things (algorithms) that can select a problem aircraft from an airspace
type Selector interface {
	String() string

	// Pick out the noise-making flight from the array, and a oneline explanation. If none
	// was identified, returns nil (but the string will explain why).
	// The aircraft slice is initially sorted by Dist3.
	Identify(pos geo.Latlong, elev float64, aircraft []Aircraft) (*Aircraft, string)
}

// SelectorNames is the list of selectors we want users to be able to pick from
var SelectorNames = []string{"conservative","lowest"}

func NewSelector(name string) Selector {
	switch name {
	case "random": return AlgoRandom{}
	case "conservative": return AlgoConservativeNoCongestion{}
	case "lowest": return AlgoLowestBreaksTies{}
	default: return AlgoRandom{}
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
func (a AlgoConservativeNoCongestion)String() string { return "Conservative, needs no congestion" } 
func (a AlgoConservativeNoCongestion)Identify(pos geo.Latlong, elev float64, in []Aircraft) (*Aircraft,string) {
	if (in[0].Dist3 >= 12.0) {
		return nil, "not picked; 1st closest was too far away (>12KM)"
	} else if (len(in) == 1) || (in[1].Dist3 - in[0].Dist3) > 4.0 {
		return &in[0], "selected 1st closest"
	} else {
		return nil, "not picked; 2nd closest was too close to 1st (<4KM)"
	}
}

// The original, "no congestion allowed" heuristic ...
type AlgoLowestBreaksTies struct{}
func (a AlgoLowestBreaksTies)String() string { return "Picks lowest if there's congestion [EXPERIMENTAL]" } 
func (a AlgoLowestBreaksTies)Identify(pos geo.Latlong, elev float64, in []Aircraft) (*Aircraft,string) {
	if (in[0].Dist3 >= 12.0) {
		return nil, "not picked; 1st closest was too far away (>12KM)"
	} else if (len(in) == 1) || (in[1].Dist3 - in[0].Dist3) > 4.0 {
		return &in[0], "selected 1st closest"
	} else {
		// Congestion.
		hDist := in[1].Dist3 - in[0].Dist3
		if math.Abs(in[1].Altitude - in[0].Altitude) > 2000 {
			if in[1].Altitude > in[0].Altitude {
				return &in[0], fmt.Sprintf("selected 1st closest (2nd was only %.1fKM away from 1st, but was %.0ft higher)", hDist, in[1].Altitude-in[0].Altitude)
			} else {
				return &in[1], fmt.Sprintf("selected 2nd closest (2nd was only %.1fKM away from 1st, but was %.0ft lower)", hDist, in[0].Altitude-in[1].Altitude)
			}
		} else {
			return nil, fmt.Sprintf("not picked; 2nd closest was too close to 1st (%.1fKM away, and %.0fft away)",
				hDist, in[1].Altitude-in[0].Altitude)
		}
	}
}
