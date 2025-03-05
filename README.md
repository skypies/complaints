How to run a local instance
---------------------------

Download and run the site locally:
```sh
go get github.com/skypies/complaints/app/frontend                           # pulls down dependencies
mv complaints/config/test-values.go.sample complaints/config/test-values.go # setup test config
cd $GOPATH/github.com/skypies/complaints
go run ./app/frontend/*go`                                                  # build & run locally
```
* Look at <http://localhost:8080/> (appengine admin panel is <http://localhost:8000/>)

Deploy an instance of the site to your google cloud project:
```sh
cd $GOPATH/github.com/skypies/complaints
export YOURPROJECT=serfr0-1000
gcloud datastore --project=$YOURPROJECT create-indexes app/index.yaml
gcloud app deploy --project=$YOURPROJECT app/dispatch.yaml
gcloud app deploy --project=$YOURPROJECT app/queue.yaml
gcloud app deploy --project=$YOURPROJECT app/cron.yaml


// old and deprecated
// gcloud app deploy --project=$YOURPROJECT --version=1 app/frontend
// gcloud app deploy --project=$YOURPROJECT --version=1 app/overnight

// the New Way, as of 2025.03 (need to tick/tock names, can't overwrite the running one)
cd $GOPATH/github.com/skypies/complaints
cd app/frontend  && gcloud app deploy --project=$YOURPROJECT --version=1tock --appyaml=app.yaml

cd $GOPATH/github.com/skypies/complaints
cd app/overnight && gcloud app deploy --project=$YOURPROJECT --version=1tock --appyaml=app.yaml

```

The `test-values.go.sample` sample file has no passwords in, so
Facebook login won't be working.

Run the command line tool:
```sh
cd $GOPATH/github.com/skypies/complaints
export GOOGLE_APPLICATION_CREDENTIALS=~/auth/token.json
go run cmd/cdb/cdb/go -h
```
