package complaints

/*

** PROBLEMO: this req comes in, but we deem it to have no session, despite the happy cookie ?

199.27.128.170 - bsmartt3 [21/Oct/2016:06:17:32 -0700] "POST /add-complaint HTTP/1.1" 302 53 http://stop.jetnoise.net/ "Mozilla/5.0 (iPad; CPU OS 9_3_5 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13G36 Safari/601.1" "stop.jetnoise.net" ms=95 cpu_ms=4 cpm_usd=5.9229999999999995e-9 loading_request=0 instance=00c61b117cc12b7a94f659ee747791297ecfefa7ebfe80b410d0fbe8ffce app_engine_release=1.9.46 trace_id=-

06:17:32.839
session was empty; no cookie ?

06:17:32.839
session: &sessions.Session{ID:"", Values:map[interface {}]interface {}{}, Options:(*sessions.Options)(0xc010943440), IsNew:true, store:(*sessions.CookieStore)(0xc01052a400), name:"serfr0"}

06:17:32.839
cookie: ACSID=~AJKiYcFtWT5Pdol8GYGHSPh1VvlPUY1LFcbBgB0aN5moSNSY5SOjH_VCvluEVfy4weyAyDXcVnd2qV-609njZrr7sGD6v4MTijoUY6Y8sorKt223TjfUmZbA-HkZ08G4EaUHklxX03DntTqYXXfHL8hdkcHcLwu9Xxnp4UWGdY74C7prG3wky2UysEe-8rfW655Ub8yISOUI0bsjtvlMmAtuRl-QuMYM9e6TcpAbfir24tP42VKXCdKFBYsh1DSwF4Z1oNBivFsfdjyjmQ-RptvnpAaUaoYlexP63GqP1GB8XX_HyiA3MMWKjSYAsIckVGUdFQifkdzF

06:17:32.839
cookie: serfr0=MTQ3NDM0NDg1N3xEdi1oQkFFQ182SUFBUkFCRUFBQVlmLWlBQUlHYzNSeWFXNW5EQWNBQldWdFlXbHNCbk4wY21sdVp3d1VBQkppYzIxaGNuUjBNMEJuYldGcGJDNWpiMjBHYzNSeWFXNW5EQWdBQm5SemRHRnRjQVp6ZEhKcGJtY01GZ0FVTWpBeE5pMHdPUzB5TUZRd05Eb3hORG94TjFvPXwpXlHKjWzcyyPSYjfnGN0_rfKS5vgOwG1gXtMg9aSsnA==

06:17:32.839
cookie: __cfduid=d0322a5866de9d856b4a11400759033f91464740745

06:17:32.839
req: POST /add-complaint HTTP/1.1
Host: stop.jetnoise.net
Connection: close
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,* /*;q=0.8
Accept-Language: en-us
Cf-Connecting-Ip: 162.251.185.174
Cf-Ipcountry: US
Cf-Ray: 2f54fd877077289a-SJC
Cf-Visitor: {"scheme":"http"}
Content-Type: application/x-www-form-urlencoded
Cookie: ACSID=~AJKiYcFtWT5Pdol8GYGHSPh1VvlPUY1LFcbBgB0aN5moSNSY5SOjH_VCvluEVfy4weyAyDXcVnd2qV-609njZrr7sGD6v4MTijoUY6Y8sorKt223TjfUmZbA-HkZ08G4EaUHklxX03DntTqYXXfHL8hdkcHcLwu9Xxnp4UWGdY74C7prG3wky2UysEe-8rfW655Ub8yISOUI0bsjtvlMmAtuRl-QuMYM9e6TcpAbfir24tP42VKXCdKFBYsh1DSwF4Z1oNBivFsfdjyjmQ-RptvnpAaUaoYlexP63GqP1GB8XX_HyiA3MMWKjSYAsIckVGUdFQifkdzF; serfr0=MTQ3NDM0NDg1N3xEdi1oQkFFQ182SUFBUkFCRUFBQVlmLWlBQUlHYzNSeWFXNW5EQWNBQldWdFlXbHNCbk4wY21sdVp3d1VBQkppYzIxaGNuUjBNMEJuYldGcGJDNWpiMjBHYzNSeWFXNW5EQWdBQm5SemRHRnRjQVp6ZEhKcGJtY01GZ0FVTWpBeE5pMHdPUzB5TUZRd05Eb3hORG94TjFvPXwpXlHKjWzcyyPSYjfnGN0_rfKS5vgOwG1gXtMg9aSsnA==; __cfduid=d0322a5866de9d856b4a11400759033f91464740745
Origin: http://stop.jetnoise.net
Referer: http://stop.jetnoise.net/
User-Agent: Mozilla/5.0 (iPad; CPU OS 9_3_5 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13G36 Safari/601.1
X-Appengine-City: ?
X-Appengine-Citylatlong: 0.000000,0.000000
X-Appengine-Country: US
X-Appengine-Default-Namespace: jetnoise.net
X-Appengine-Region: ?
X-Cloud-Trace-Context: 0af7e246dad0e9fe88dd95c6176215bb/14895145604500146119
X-Forwarded-For: 162.251.185.174
X-Forwarded-Proto: http
X-Google-Apps-Metadata: domain=jetnoise.net,host=stop.jetnoise.net
X-Zoo: app-id=serfr0-1000,domain=jetnoise.net,host=stop.jetnoise.net

browser_uuid=5214ec17-d9fb-4b60-b8d7-4970d1173762&browser_name=Safari&browser_version=9&browser_vendor=Apple+Computer%2C+Inc.&browser_platform=iPad&content=&activity=&loudness=1


** BUT, prev seen cookie was different:

23:43:29.441
[root_002]  0.000052 cookie: serfr0=MTQ3NjExMjA3NHxEdi1oQkFFQ182SUFBUkFCRUFBQVl2LWlBQUlHYzNSeWFXNW5EQWNBQldWdFlXbHNCbk4wY21sdVp3d1ZBQk5yWlhOamJ6RXlNamRBWjIxaGFXd3VZMjl0Qm5OMGNtbHVad3dJQUFaMGMzUmhiWEFHYzNSeWFXNW5EQllBRkRJd01UWXRNVEF0TVRCVU1UVTZNRGM2TlRSYXygQVCIvUQAywzGYgoN7_xaknpIi7N8A98RwF4m7gL78Q==

** Should start logging: details about decoded session (and a csum of decoding key)
** also, cookies being written (is this possible ?)


*/

/* Common code for pulling out a user session cookie, populating a Context, etc.
 * Users that aren't logged in will be redirected to the specified URL.

func init() {
  http.HandleFunc("/deb", HandleWithSession(debHandler, "/")) // If no cookie, redirects to "/"
}

func debHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sesh,ok := GetUserSession(ctx)
	str := fmt.Sprintf("OK\nemail=%s, ok=%v\n", sesh.Email, ok) 
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

 */

import(
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"github.com/skypies/complaints/sessions"
)

// Pretty much all handlers should expect to be able to pluck this object out of their Context
type UserSession struct {
	Email        string     // case sensitive, sadly
	CreatedAt    time.Time  // when the user last went through the OAuth2 dance
	hasCreatedAt bool
}

func (us UserSession)HasCreatedAt() bool { return us.hasCreatedAt }

// To prevent other libs colliding in the context.Value keyspace, use this private key
type contextKey int
const sessionEmailKey contextKey = 0

type baseHandler    func(http.ResponseWriter, *http.Request)
type contextHandler func(context.Context, http.ResponseWriter, *http.Request)

func HandleWithSession(ch contextHandler, ifNoSessionRedirectTo string) baseHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx,_ := context.WithTimeout(appengine.NewContext(r), 55 * time.Second)

		session,err := sessions.Get(r)
		if err != nil {
			log.Errorf(ctx, "session.Get failed with err: %v", err)
		}
		
		if strings.HasPrefix(r.UserAgent(), "Google") {
			// Robot - do nothing

		} else if session.Values["email"] == nil {
			reqBytes,_ := httputil.DumpRequest(r, true)
			log.Errorf(ctx, "session was empty; no cookie ?")
			log.Errorf(ctx, fmt.Sprintf("key was len %d", len(kSessionsKey)))
			log.Errorf(ctx, "session: %d values", len(session.Values))
			for k,v := range session.Values {
				log.Errorf(ctx, "* %s: %s", k, v)
			}
			for _,c := range r.Cookies() {
				log.Errorf(ctx, "cookie: %s", c)
			}
			log.Errorf(ctx, "req: %s", reqBytes)

			// Bleargh
			sessions.Init(kSessionsKey,kSessionsPrevKey)
			session,err = sessions.Get(r)
			log.Errorf(ctx, "session2: %d values (err=%v)", len(session.Values), err)
			for k,v := range session.Values {
				log.Errorf(ctx, "*2 %s: %s", k, v)
			}
			
			// If we have a URL to redirect to, in cases of no session, then do it
			if ifNoSessionRedirectTo != "" {
				http.Redirect(w, r, ifNoSessionRedirectTo, http.StatusFound)
				return
			}

		} else {
			sesh := UserSession{Email: session.Values["email"].(string)}

			if session.Values["tstamp"] != nil {
				tstampStr := session.Values["tstamp"].(string)
				tstamp,_ := time.Parse(time.RFC3339, tstampStr)
				sesh.CreatedAt = tstamp
				sesh.hasCreatedAt = true // time.IsZero seems useless
			}
			
			ctx = context.WithValue(ctx, sessionEmailKey, sesh)
		}

		// Call the underlying handler, with our shiny context
		ch(ctx, w, r)
	}
}

// Underlying handlers should call this to get their session object
func GetUserSession(ctx context.Context) (UserSession,bool) {
	sesh, ok := ctx.Value(sessionEmailKey).(UserSession)
	return sesh, ok
}
