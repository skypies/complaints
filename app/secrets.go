package main

import "github.com/skypies/complaints/config"

// Setup some 'constants' across the serfr0 package, pulling secrets from the
// config lookup.
var (
	kGoogleMapsAPIKey = config.Get("googlemaps.apikey")
	kFacebookAppId = config.Get("facebook.appid")
	kFacebookAppSecret = config.Get("facebook.appsecret")

	kFlightawareAPIUsername = config.Get("flightaware.username")
	kFlightawareAPIKey = config.Get("flightaware.key")

	kSessionsKey = config.Get("sessions.key")
	kSessionsPrevKey = config.Get("sessions.prevkey")
)
