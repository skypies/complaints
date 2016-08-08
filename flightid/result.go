package flightid

import()

// Result has all the data retrieved from our attempt to identify the aircraft overhead
type Result struct {
	Flight     *Aircraft
	Err         error

	All      []*Aircraft
	Filtered []*Aircraft

	Debug       string
}

