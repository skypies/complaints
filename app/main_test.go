package complaints

// cd complaints/app && goapp test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"
)

var inst aetest.Instance
func init() {
	inst,_ = aetest.NewInstance(&aetest.Options{StronglyConsistentDatastore: true})
} // should call inst.Close, but eh

// {{{ vals

func vals(s ...string) url.Values {
	v := url.Values{}
	for i:=0; i<len(s); i+=2 {
		v[s[i]] = []string{s[i+1]}
	}
	return v
}

// }}}
// {{{ newr

func newr(t *testing.T, path string, vals url.Values) *http.Request {
	method := "POST"
	bodyParams := bytes.NewBufferString(vals.Encode())
	r,err := inst.NewRequest(method, path, bodyParams)

	r.Header.Add("Content-Type", "application/x-www-form-urlencoded") // ffs
	
	if err != nil { t.Fatal(err) }
	return r
}

// }}}
// {{{ call

func call(t *testing.T, r *http.Request, handler baseHandler) (int,http.Header,string) {
	w := httptest.NewRecorder()
	handler(w,r)

	if w.Code >= 400 {
		t.Errorf("Request %v returned [%d], not [200]\n", r, w.Code)
	}

	return w.Code, w.Header(), w.Body.String()
}

// }}}

// {{{ withSpoofedSession

// replacement for WithSession, that injects a spoofed usersession instead of
// doing all the cookie crypto stuff.
func withSpoofedSession(sesh ui.UserSession, ch contextHandler) baseHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx,_ := context.WithTimeout(appengine.NewContext(r), 55 * time.Second)
		ctx = ui.SetUserSession(ctx, sesh)

		ch(ctx, w, r)
	}
}

// }}}
// {{{ {with,without}User

// Convenience functions to run contexthandlers with/without a spoofed user session
func withoutUser(ch contextHandler) baseHandler { return ui.WithCtxSession(ch, nil) }
func withUser(user string, ch contextHandler) baseHandler {
	sesh := ui.UserSession{Email: user}
	return withSpoofedSession(sesh, ch)
}

// }}}

// {{{ contains

func contains(t *testing.T, r *http.Request, handler baseHandler, re string) []string {
	code,hdrs,body := call(t, r, handler)

	matches := regexp.MustCompile(re).FindStringSubmatch(body)
	if len(matches) == 0 {
		t.Errorf("Response:-\nHTTP %d\n%s\n%s\n--\nregexp '%s' didn't match\n", code,hdrs,body, re)
	}
	return matches
}

// }}}
// {{{ redirects

func redirects(t *testing.T, r *http.Request, handler baseHandler, dest string) {
	code,hdrs,body := call(t, r, handler)

	if code < 300 {
		t.Errorf("Response:-\nHTTP %d\n%s\n%s\n--\nwas not a redirect\n", code,hdrs,body)
	}

	if hdrs.Get("Location") != dest {
		t.Errorf("Response:-\nHTTP %d\n%s\n%s\n--\ndidn't redirect to %s\n", code,hdrs,body, dest)
	}
}

// }}}

// {{{ TestHappyPath

func TestHappyPath(t *testing.T) {
	// no sesh should render the splash page
	contains(t, newr(t, "/", vals()), withoutUser(rootHandler), "Login just once")

	// a logged in but brand new user should redirect to profile edit
	user := "tester@t.io"
	redirects(t, newr(t, "/", vals()), withUser(user,rootHandler), "/profile")

	// This should build a profile
	vProfile := vals(
		"Lat", "37.6188172",	// KLatlongSFO
		"Long", "-122.3754281",

		"Elevation", "250.0",
		"CallerCode", "ABC123",
		"FullName", "Tester Human",
		"AddrNumber", "1",
		"AddrStreet", "Scotts Valley Drive",
		"AddrCity", "Scotts Valley",
		"AddrState", "CA",
		"AddrZip", "95066",
		"AddrCountry", "United States",
		"SelectorAlgorithm", "conservative")
	redirects(t, newr(t, "/profile-update", vProfile), withUser(user,profileUpdateHandler), "/")

	// now, we should get to the main page ...
	contains(t, newr(t, "/", vals()), withUser(user,rootHandler), "Jet noise overhead ?")

	// ... and we should be able to submit a complaint
	description := "Lemons"
	vComplaint := vals(
		"content", description,
		"loudness", "1",
	)
	redirects(t, newr(t, "/add-complaint", vComplaint), withUser(user,addComplaintHandler), "/")

	// ... and it should show up on the main page
	contains(t, newr(t, "/", vals()), withUser(user,rootHandler), description)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
