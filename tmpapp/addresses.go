package main

import (
	"fmt"
	"net/http"
	"strings"

	//"github.com/skypies/util/gcp/tasks"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	// http.HandleFunc("/tmp/addresses", addressesHandler)
}

// {{{ addressesHandler

// Grab all users, and enqueue them for batch processing
func addressesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	cps,err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil {
		cdb.Errorf("upgradeHandler: getprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := fmt.Sprintf("Found %d profiles\n\n", len(cps))

	good,noStreet,noCity,noStr := 0,0,0,0

	addrs := []string{}
	
	for _,cp := range cps {
		addrS := cp.StructuredAddress

		if addrS.Street == "" {
			noStreet++

			if r.FormValue("fix") != "" {
				addrS = cp.GetStructuredAddress() // May call the google maps geocoder api
				cdb.PersistProfile(cp)
			}

			addrs = append(addrs, fmt.Sprintf("{%s} %#v", cp.Address, addrS))

		} else if addrS.City == "" {
			noCity++
			str += fmt.Sprintf("{{%s}} %#v\n", cp.Address, addrS)
		} else if cp.Address == "" {
			noStr++
		} else {
			good++
		}
	}

	str += fmt.Sprintf("\n\ngood: %d\nnoCity: %d\nnoStreet: %d\nnoStr: %d\n", good, noCity, noStreet, noStr)

	str += "\n\n--\n" + strings.Join(addrs, "\n") + "\n--\n"
	
	w.Write([]byte(fmt.Sprintf("OK\n\n%s", str)))
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
