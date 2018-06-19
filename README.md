How to run a local instance
---------------------------

Prerequisites:
* the [Go programming language](https://golang.org/dl/)
* the [Go appengine SDK](https://cloud.google.com/appengine/docs/go/), and add it to your `$PATH`
* define your Go workspace: `export GOPATH=~/go`

Download and run the site locally:
* `go get github.com/skypies/complaints/app` (pulls down all dependencies)
* `mv complaints/config/test-values.go.sample complaints/config/test-values.go` (setup test config)
* `dev_appserver.py $GOPATH/github.com/skypies/complaints/app` (build & run locally)
* Look at <http://localhost:8080/> (appengine admin panel is <http://localhost:8000/>)

Deploy an instance of the site to your google cloud project:
* `cd $GOPATH/github.com/skypies/complaints`
* `export YOURPROJECT=serfr0-1000`
* `gcloud datastore --project=$YOURPROJECT create-indexes app/index.yaml`
* `gcloud app deploy --project=$YOURPROJECT --version=one app`
* `gcloud app deploy --project=$YOURPROJECT --version=one backend`

The `test-values.go.sample` sample file has no passwords in, so
Facebook login won't be working.

