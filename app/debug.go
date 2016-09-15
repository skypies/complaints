package complaints

import (	
	"fmt"
	"net/http"
	"time"
	
	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/flightid"
	"github.com/skypies/complaints/fr24"
)

func init() {
	http.HandleFunc("/deb", debHandler)
	http.HandleFunc("/deb2", HandleWithSession(debSessionHandler, "/"))
	http.HandleFunc("/deb3", HandleWithSession(debSessionHandler, ""))
	http.HandleFunc("/optouts", optoutHandler)
}

func debSessionHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sesh,ok := GetUserSession(ctx)
	str := fmt.Sprintf("OK\nctx = [%T] %v\nemail=%s, ok=%v\n", ctx, ctx, sesh.Email, ok) 
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

func debHandler(w http.ResponseWriter, r *http.Request) {
	pos := sfo.KLatlongSFO
	ctx := req2ctx(r)
	client := complaintdb.NewDB(ctx).HTTPClient()

	fr := fr24.Fr24{Client: client}
	if err := fr.EnsureHostname(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	overheadFr24 := flightid.Aircraft{}
	allFr24,filtFr24,debugFr24,err := fr.FindAllOverhead(pos, &overheadFr24, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fr24Str := fmt.Sprintf("== %2d/%2d : %40.40s", len(filtFr24), len(allFr24), overheadFr24.String())

	f,all,filt,err,debug := flightid.FindAllOverhead(client, pos, 0, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	skypiStr := fmt.Sprintf("== %2d/%2d : %40.40s", len(filt), len(all), f)
	str := fmt.Sprintf("== fr24  %s\n== skypi %s\n\n==== fr24 ====\n%s\n==== SkyPi ====\n%s",
		fr24Str, skypiStr, debugFr24, debug)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

func optoutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	users,err := cdb.GetComplainersCurrentlyOptedOut()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	str := fmt.Sprintf("OK! (%d results)\n\n", len(users))
	for user,_ := range users {
		str += fmt.Sprintf(" %s\n", user)
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

// countHackHandler will append a new complaint total to the daily counts object.
// These are sorted elsewhere, so it's OK to 'append' out of sequence.
// Note no deduping is done; if you want that, add it here.
func countHackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	cdb.AddDailyCount(complaintdb.DailyCount{
		Datestring: "2016.06.22",
		NumComplaints: 6555,
		NumComplainers: 630,
	})

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK!\n"))	
}

func perftesterHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	tStart := time.Now()
	str := ""
	debugf := func(step string, fmtstr string, varargs ...interface{}) {
		payload := fmt.Sprintf(fmtstr, varargs...)
		outStr := fmt.Sprintf("[%s] %9.6f %s", step, time.Since(tStart).Seconds(), payload)
		log.Debugf(ctx, outStr)
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
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	tStart := time.Now()
	
	start,end := date.WindowForYesterday()
	keys,err := cdb.GetComplaintKeysInSpan(start,end)

	str := fmt.Sprintf("OK\nret: %d\nerr: %v\nElapsed: %s\ns: %s\ne: %s\n",
		len(keys), err, time.Since(tStart), start, end)
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
