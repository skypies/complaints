package complaints

import (
	"net/http"
	//"appengine"
	//"github.com/skypies/complaints/complaintdb"
)

// The flatpage template will get a userprofile as dot, in case that's handy.
// To add a new flat page:
//  1. Add a HandleFunc, as per below
//  2. Add a new template, and have it match the URL, e.g. {{define "/f/foobar"}}

func init() {
	http.HandleFunc("/about", flat)
	http.HandleFunc("/down", flat)
}

func flat(w http.ResponseWriter, r *http.Request) {
	//c := appengine.NewContext(r)
	//cdb := complaintdb.ComplaintDB{C: appengine.NewContext(r)}

	pagename := r.URL.Path

	if err := templates.ExecuteTemplate(w, pagename, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
