How to run a local instance
---------------------------

Prerequisites:
* the [Go programming language](https://golang.org/dl/)
* the [Go appengine SDK](https://cloud.google.com/appengine/docs/go/), and add it to your `$PATH`
* define your Go workspace: `export GOPATH=~/go`

Download and run the site locally:
* `go get github.com/skypies/complaints/app` (pulls down all dependencies)
* `mv complaints/config/test-values.go.sample complaints/config/test-values.go` (setup test config)
* `goapp serve $GOPATH/github.com/skypies/complaints/app` (build & run locally)
* Look at <http://localhost:8080/> (appengine admin panel is <http://localhost:8000/>)

The `test-values.go.sample` sample file has no passwords in, so
Facebook login and flightaware flightpath backfill won't be working.

