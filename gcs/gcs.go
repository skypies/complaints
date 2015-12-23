package gcs

/* http://storage-test-1151.appspot.com/

Blindly following all structions made it work !

Had to create a bucket, and put its name into app.go

I think a key step is 
https://cloud.google.com/appengine/docs/go/googlecloudstorageclient/download
- go get -u google.golang.org/appengine/...

But perhaps the 'gcloud auth init' step was the fix to error 9 / UNKNOWN_SCOPE ?

See ~/gcs/tmp etc; be v careful with GOPATH and PATH.
 
https://cloud.google.com/appengine/docs/go/googlecloudstorageclient/
https://cloud.google.com/appengine/docs/go/googlecloudstorageclient/download
https://cloud.google.com/appengine/docs/go/googlecloudstorageclient/getstarted

https://cloud.google.com/sdk/gcloud/

 */

/* GCS on Go appears to be broken: API error 9 (app_identity_service: UNKNOWN_SCOPE)

import (
	"io"
	"net/http"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/cloud/storage"
)

func OpenRW(r *http.Request, bucketname string, filename string) io.Writer {
	ctx := appengine.NewContext(r)
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf(ctx, "failed to get a client: %v", err)
		return nil
	}
	defer client.Close()

	bucket := client.Bucket(bucketname)
	wc := bucket.Object(filename).NewWriter(ctx)
	wc.ContentType = "text/plain" // CSV?
	
	return io.Writer(wc)
}

*/
