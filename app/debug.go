package complaints

import (	
	"fmt"
	"net/http"
	"time"
	
	"appengine"
	"appengine/urlfetch"

	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/fr24"
)

func init() {
	// http.HandleFunc("/debfr24", debugHandler2)
	//http.HandleFunc("/counthack", debugHandler4)
	http.HandleFunc("/debbksv", debugHandler3)
}

func debugHandler3(w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewComplaintDB(r)
	tStart := time.Now()
	
	start,end := date.WindowForYesterday()
	keys,err := cdb.GetComplaintKeysInSpan(start,end)

	str := fmt.Sprintf("OK\nret: %d\nerr: %v\nElapsed: %s\ns: %s\ne: %s\n",
		len(keys), err, time.Since(tStart), start, end)
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

// Hack up the counts.
func debugHandler4(w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewComplaintDB(r)

	cdb.AddDailyCount(complaintdb.DailyCount{
		Datestring: "2016.04.12",
		NumComplaints: 10231,
		NumComplainers: 621,
	})
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
