package main

import "github.com/skypies/complaints/config"

// Setup some 'constants' across the serfr0 package, pulling secrets from the
// config lookup.
var (
	kGoogleMapsAPIKey = config.Get("googlemaps.apikey")
	kFacebookAppId = config.Get("facebook.oauth2.appid")
	kFacebookAppSecret = config.Get("facebook.oauth2.secret")

	kFlightawareAPIUsername = config.Get("flightaware.username")
	kFlightawareAPIKey = config.Get("flightaware.key")

	kSessionsKey = config.Get("sessions.key")
	kSessionsPrevKey = config.Get("sessions.prevkey")
)
