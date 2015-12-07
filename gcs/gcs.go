package gcs

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
