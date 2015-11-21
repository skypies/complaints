The complaints website

How to get it running
---------------------

Prerequisites:
* the [Go programming language](https://golang.org/dl/)
* the [Go appengine SDK](https://cloud.google.com/appengine/docs/go/), and add it to your `$PATH`
* define your Go workspace: `export GOPATH=/home/you/go_workspace`.

Download and run the site locally:
* `goapp get github.com/skypies/complaints/app` (pulls down all dependencies
* `goapp serve $GOPATH/github.com/skypies/complaints/app` (build & run locally)
* Look at <http://localhost:8080/> (admin panel is <http://localhost:8000/>)

A few things depend on credentials that aren't stored in github:
* realtime overhead flight lookups
* facebook login
* fligthaware flightpath backfill

