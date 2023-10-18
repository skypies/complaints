module github.com/skypies/complaints

go 1.13

require (
	cloud.google.com/go/bigquery v1.56.0
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/mailjet/mailjet-apiv3-go v0.0.0-20190724151621-55e56f74078c
	github.com/paulmach/go.geojson v1.5.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/skypies/adsb v0.1.0 // indirect
	github.com/skypies/flightdb v0.1.3
	github.com/skypies/geo v0.0.0-20180901233721-9d4f211f3066
	github.com/skypies/pi v0.1.2
	github.com/skypies/util v0.1.31
	golang.org/x/net v0.17.0
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	google.golang.org/api v0.147.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/grpc v1.59.0 // indirect
	googlemaps.github.io/maps v0.0.0-20191014172202-ce2e58e026c5
)

// replace github.com/skypies/flightdb => ../flightdb

// replace github.com/skypies/util => ../util
