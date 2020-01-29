package main

import(
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	//"net/url"
	//"strings"
	//"github.com/skypies/util/gcp/tasks"

	"golang.org/x/net/context"
	
	hackbackend "github.com/skypies/complaints/backend"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/config"
	"github.com/skypies/complaints/login" // autoregisters some handlers
	hackui "github.com/skypies/complaints/ui" // for login handler session creation
)

var(
	LocationID = "us-central1" // This is "us-central" in appengine-land, needs a 1 for cloud tasks
	ProjectID = "serfr0-1000"
	QueueName = "submitreports"
)

func init() {
	//http.HandleFunc("/tmp/args", argsHandler)
	//http.HandleFunc("/tmp/hello", helloHandler)
	//http.HandleFunc("/tmp/taski", taskHandler)

	http.HandleFunc("/tmp/submissions/debug", complaintdb.SubmissionsDebugHandler)
	http.HandleFunc("/tmp/submissions/debug2", complaintdb.SubmissionsDebugHandler2)
	http.HandleFunc("/tmp/cdb/comp/debug", complaintdb.ComplaintDebugHandler)

	http.HandleFunc("/tmp/login",         hackLoginHandler)

	hackui.InitSessionStore(config.Get("sessions.key"), config.Get("sessions.prevkey"))
	
	login.OnSuccessCallback = func(w http.ResponseWriter, r *http.Request, email string) error {
		hackui.CreateSession(r.Context(), w, r, hackui.UserSession{Email:email})
		return nil
	}
	login.Host                  = "https://stop.jetnoise.net"
	login.RedirectUrlStem       = "/tmp/login" // oauth2 callbacks will register  under here
	login.AfterLoginRelativeUrl = "/" // where the user finally ends up, after being logged in
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("tmpa[mp]p listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 599 * time.Second)
	return ctx
}
func req2client(r *http.Request) *http.Client {
	return &http.Client{}
}

func hackLoginHandler(w http.ResponseWriter, r *http.Request) {

	var params = map[string]interface{}{
		"GoogleLogin": login.Goauth2.GetLoginUrl(w,r),
		"GoogleLogout": login.Goauth2.GetLogoutUrl(w,r),
		"FacebookLogin": login.Fboauth2.GetLoginUrl(w,r),
	}

	if err := hackbackend.HackTemplates.ExecuteTemplate(w, "glogin-1", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

/*
func taskHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	str := "Handling a task [[5]] !!\n--\n"
	for k,v := range r.Form {
		str += fmt.Sprintf(" *  %s: %s\n", k, strings.Join(v, ", "))
	}
	str += "--\n"
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
	log.Printf(str)
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	str := "Yes, hello [[5]]\n"

	handlerUri := "/tmp/taski"

	params := url.Values{}
	params.Add("foo", "bar")
	params.Set("baz", "quux")

	delay := time.Second * 120
	
	task,err := tasks.SubmitAETask(ctx, ProjectID, LocationID, QueueName, delay, handlerUri, params)
	
	str += fmt.Sprintf("\ntime: %s\nerr: %v\n\ntask: %#v\n\n", time.Now(), err, task)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}
*/

/*

func argsHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	str := "Args:-\n"
	for k,v := range r.Form {
		str += fmt.Sprintf(" * %-40.40s : %s\n", k, v)
	}
	str += "--\n"
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}
*/
