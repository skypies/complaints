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
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/fr24"
)

func init() {
	// http.HandleFunc("/debfr24", debugHandler2)
	//http.HandleFunc("/counthack", debugHandler4)
	http.HandleFunc("/debbksv", debugHandler3)
	http.HandleFunc("/perftester", perftesterHandler)
}

func perftesterHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	tStart := time.Now()
	str := ""
	debugf := func(step string, fmtstr string, varargs ...interface{}) {
		payload := fmt.Sprintf(fmtstr, varargs...)
		outStr := fmt.Sprintf("[%s] %9.6f %s", step, time.Since(tStart).Seconds(), payload)
		ctx.Debugf(outStr)
		str += "* " + outStr + "\n"
	}

	debugf("pt_001", "starting")

	stats := map[string]*complaintdb.DailyCount{}
	dailys := []complaintdb.DailyCount{}
	counts := []types.CountItem{}

	debugf("pt_002", "populating")
	t := time.Now().AddDate(0,-6,0)
	for i:=0; i<150; i++ {
		t = t.AddDate(0,0,1)
		dailys = append(dailys, complaintdb.DailyCount{t.Format("2006.01.02"),i,i,false,false})
		item := complaintdb.DailyCount{t.Format("2006.01.02"),i,i,false,false}
		stats[date.Datestring2MidnightPdt(item.Datestring).Format("Jan 02")] = &item
	}
	debugf("pt_005", "populating all done")


	pdt, _ := time.LoadLocation("America/Los_Angeles")
	dateNoTimeFormat := "2006.01.02"
	arbitraryDatestring2MidnightPdt := func(s string, fmt string) time.Time {
		t,_ := time.ParseInLocation(fmt, s, pdt)
		return t
	}
	datestring2MidnightPdt := func(s string) time.Time {
		return arbitraryDatestring2MidnightPdt(s, dateNoTimeFormat)
	}
	_=datestring2MidnightPdt
	
	debugf("pt_010", "daily stats loaded (%d dailys, %d stats)", len(dailys), len(stats))
	for _,daily := range dailys {
		// cdb.C.Infof(" -- we have a daily: %#v", daily)
		//key := datestring2MidnightPdt(daily.Datestring).Format("Jan 02")
		item := types.CountItem{
			//Key: key, //fmt.Sprintf("Jan %02d", j+1), //daily.Timestamp().Format("Jan 02"),
			Key: daily.Timestamp().Format("Jan 02"),
			Count: daily.NumComplaints,
		}
		if dc,exists := stats[item.Key]; exists {
			item.TotalComplainers = dc.NumComplainers
			item.TotalComplaints = dc.NumComplaints
			item.IsMaxComplainers = dc.IsMaxComplainers
			item.IsMaxComplaints = dc.IsMaxComplaints
		}
		//counts = append(counts, item)
	}
	debugf("pt_090", "daily stats munged (%d counts)", len(counts))

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))	
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
