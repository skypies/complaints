package main

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/skypies/util/widget"
	"github.com/skypies/complaints/pkg/complaintdb"
)

//  &user=a@b.com
//  &date=day&day=2016/05/04

func listUsersComplaintsHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)

	n := widget.FormValueIntWithDefault(r, "n", 100)
	userToList := r.FormValue("user")
	if userToList == "" {
		http.Error(w, "needs CGI arg &user=foo@bar.com", http.StatusInternalServerError)
		return
	}

	str := fmt.Sprintf("<html><body><h1>%d complaints for %s</h1><table>\n", n, userToList)	

	q := cdb.CQByEmail(userToList)

	if r.FormValue("date") != "" {
		start,end,_ := widget.FormValueDateRange(r)
		q = q.ByTimespan(start,end)
		str += fmt.Sprintf("<tr><td>S:%v<br/>E:%v</td></tr>\n", start, end)
	}

	q = q.OrderTimeDesc().Limit(n)

	complaints,err := cdb.LookupAll(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(complaints) < n { n = len(complaints) }

	for _,c := range complaints {
		url := "/complaint-updateform?k="+c.DatastoreKey
		str += fmt.Sprintf("<tr><td><a href=\"%s\">%s</a></td></tr>", url, c)
	}
	str += fmt.Sprintf("</table><p><tt>%s</tt</p></body></html>\n", q.String())

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(str))
}
