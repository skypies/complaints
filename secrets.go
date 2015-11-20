package serfr0

import "config"

// Setup some 'constants' across the serfr0 package, pulling secrets from the
// config lookup.
var (
	kGoogleMapsAPIKey = config.Get("googlemaps.apikey")
)
