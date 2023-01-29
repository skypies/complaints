module github.com/skypies/complaints

go 1.13

require (
	cloud.google.com/go/bigquery v1.8.0
	github.com/google/uuid v1.1.1 // indirect
	github.com/mailjet/mailjet-apiv3-go v0.0.0-20190724151621-55e56f74078c
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/skypies/flightdb v0.1.2
	github.com/skypies/geo v0.0.0-20180901233721-9d4f211f3066
	github.com/skypies/pi v0.1.0
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9
	googlemaps.github.io/maps v0.0.0-20191014172202-ce2e58e026c5
)

require github.com/skypies/util v0.1.29

// replace github.com/skypies/util v0.1.28 => ../util
