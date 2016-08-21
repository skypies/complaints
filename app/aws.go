package complaints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/aws-iot", awsIotHandler)
}

// Comes from this lambda: https://www.losant.com/blog/getting-started-with-aws-iot-button-losant
// {"clickType":"SINGLE", "serialNumber":"DEADBEEFDEADBEEF", "batteryVoltage":"1582mV"}
type AwsIotEvent struct {
	ClickType    string `json:"clickType"`      // {SINGLE|DOUBLE|LONG}
	SerialNumber string `json:"serialNumber"`   // 16-char ascii
	Voltage      string `json:"batteryVoltage"` // "1528mV
}
func (ev AwsIotEvent)String() string {
	return fmt.Sprintf("%s@%s[%s]", ev.ClickType, ev.SerialNumber, ev.Voltage)
}

func awsIotHandler(w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(r)

	ev := AwsIotEvent{}

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
