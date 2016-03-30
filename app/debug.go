package complaints

import (	
	"fmt"
	"net/http"
	
	"appengine"
	"appengine/urlfetch"

	"github.com/skypies/geo/sfo"

	"github.com/skypies/complaints/fr24"
)

func init() {
	// http.HandleFunc("/debfr24", debugHandler2)
}

func debugHandler2(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)

	fr := fr24.Fr24{Client: client}

	if r.FormValue("h") != "" {
		fr.SetHost(r.FormValue("h"))
	} else {
		if err := fr.EnsureHostname(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	overhead := fr24.Aircraft{}
	debug,err := fr.FindOverhead(sfo.KLatlongSFO, &overhead, true)

	str := fmt.Sprintf("OK\nret: %v\nerr: %v\n--debug--\n%s\n", overhead, err, debug)		

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
