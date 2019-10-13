package main

import(
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 55 * time.Second)
	return ctx
}

func req2client(r *http.Request) *http.Client {
	return &http.Client{}
}

func dumpForm(r *http.Request) string {
	str := fmt.Sprintf("*** Form contents for {%s}:-\n", r.URL.Path)
	for k,v := range r.Form {
		str += fmt.Sprintf("  * %-20.20s : %v\n", k, v)
	}
	return str + "***\n"
}
