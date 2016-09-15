package complaints

import(
	"net/http"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"golang.org/x/net/context"
)

// Kill this stuff off ?
func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(appengine.NewContext(r), 55 * time.Second)
	return ctx
}

func req2client(r *http.Request) *http.Client {
	return urlfetch.Client(req2ctx(r))
}
