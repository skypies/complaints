package complaints

import(
	"net/http"
	"github.com/skypies/flightdb/ui"
)

func init() {
	http.HandleFunc("/map", ui.WithCtxTmpl(templates, ui.MapHandler))
}
