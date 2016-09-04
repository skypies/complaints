package complaints

import(
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"golang.org/x/net/context"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/sessions"
)

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(appengine.NewContext(r), 55 * time.Second)
	return ctx
}

func req2client(r *http.Request) *http.Client {
	return urlfetch.Client(req2ctx(r))
}

func getSessionEmail(r *http.Request) (string, error) {
	cdb := complaintdb.NewDB(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		reqBytes,_ := httputil.DumpRequest(r, true)
		cdb.Errorf("session was empty; no cookie ?")
		cdb.Errorf("session: %#v", session)
		for _,c := range r.Cookies() {
			cdb.Errorf("cookie: %s", c)
		}
		cdb.Errorf("req: %s", reqBytes)
		return "", fmt.Errorf("session was empty; no cookie ? is this browser in privacy mode ?")
	}
	email := session.Values["email"].(string)
	return email, nil
}

