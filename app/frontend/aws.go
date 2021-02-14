package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/config"
)

// Comes from this lambda: https://www.losant.com/blog/getting-started-with-aws-iot-button-losant
// {"clickType":"SINGLE", "serialNumber":"DEADBEEFDEADBEEF", "batteryVoltage":"1582mV"}
type AwsIotEvent struct {
	ClickType    string `json:"clickType"`      // {SINGLE|DOUBLE|LONG}
	SerialNumber string `json:"serialNumber"`   // 16-char ascii
	Voltage      string `json:"batteryVoltage"` // e.g. "1528mV"
	Secret       string `json:"secret"`
}
func (ev AwsIotEvent)String() string {
	return fmt.Sprintf("%s@%s[%s](%db)", ev.ClickType, ev.SerialNumber, ev.Voltage, len(ev.Secret))
}

func awsIotHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	ev := AwsIotEvent{}
	reqBytes,_ := httputil.DumpRequest(r, true)

	if false && r.FormValue("sn") != "" {
		// For debugging
		ev.ClickType = "SINGLE"
		ev.SerialNumber = r.FormValue("sn")

	} else {
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			// reqBytes,_ := httputil.DumpRequest(r, true)
			//cdb.Errorf("decode failed: %v\n%s", err, reqBytes)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	secrets := strings.Fields(config.Get("api.keys")) // The space-sep list of API keys we accept
	if len(secrets) == 0 {
		cdb.Errorf("`api.keys` secret config lookup failed ! bad config ?")
		http.Error(w, "bad secret config", http.StatusInternalServerError)
		return
	}

	secretOK := false
	for _,secret := range secrets {
		if secret == ev.Secret {
			secretOK = true
		}
	}

	if !secretOK {
		cdb.Errorf("bad secret submitted, no match in `api.keys`")
		cdb.Errorf("-> %s", reqBytes)
		http.Error(w, "bad secret submitted", http.StatusInternalServerError)
		return
	}
	
	cdb.Infof("AWS-IoT button event received: %s", ev)

	if ev.ClickType == "SINGLE" {
		complaint := types.Complaint{
			Timestamp:   time.Now(), // No point setting a timezone, it gets reset to UTC
		}

		if err := cdb.ComplainByButtonId(ev.SerialNumber, &complaint); err != nil {
			cdb.Errorf("complain failed: %v\nev=%s", err, ev)
		} else {
			cdb.Infof("complaint made: %s", complaint)
		}
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK\n" + ev.String()+"\n"))
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
