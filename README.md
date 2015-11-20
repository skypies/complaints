The complaints website

How to get it running
---------------------

1. Install the [Go programming language](https://golang.org/dl/)
2. Install the [Go appengine SDK](https://cloud.google.com/appengine/docs/go/), and add it to your `$PATH`
3. Pick a directory for your Go workspace: `export GOPATH=/home/you/go_workspace`.
4. Use appengine's goapp wrapper to git clone this repo, and its dependencies: `goapp get github.com/skypies/complaints`
(ignore the errors about unrecognized import paths for now)
5. Run a local server: `goapp serve $GOPATH/github.com/skypies/complaints`
6. Take a look at http://localhost:8080/ ! (For admin panel, http://localhost:8000/)

A few limitations of the local setup:
* flight lookups won't work (they need a local file)
* facebook login won't work (facebook refuses a localhost return URL)
