package complaints

import(
	"net/http"
	"github.com/skypies/flightdb2/ui"
)

func init() {
	http.HandleFunc("/map", ui.MapHandler)
}
