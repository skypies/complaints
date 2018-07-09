package complaints

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/skypies/util/widget"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/ui"
)

func init() {
	http.HandleFunc("/cdb/list", ui.WithCtxTlsSession(listHandler,fallbackHandler))
}

func listHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)

	n := widget.FormValueIntWithDefault(r, "n", 100)
	userToList := r.FormValue("user")
	if userToList == "" {
		http.Error(w, "needs CGI arg &user=foo@bar.com", http.StatusInternalServerError)
		return
	}

	q := cdb.CQByEmail(userToList).OrderTimeDesc().Limit(n)
	complaints,err := cdb.LookupAll(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(complaints) < n { n = len(complaints) }

	str := fmt.Sprintf("<html><body><h1>%d complaints for %s</h1><table>\n", n, userToList)	
	for _,c := range complaints {
		url := "/complaint-updateform?k="+c.DatastoreKey
		str += fmt.Sprintf("<tr><td><a href=\"%s\">%s</a></td></tr>", url, c)
	}
	str += "</table></body></html>\n"
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(str))
}
