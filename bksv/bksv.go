package bksv

// Better regex for caller code
// Better language for promoting caller code
// Flag for 'first ever' complaint ?
// device.UUIDs ... use Guy's lib, and submit.
// IDs for complainants, and complaints
// Cutover: 1st May.
// Share the published report URLs with bert et al.

// Package for posting a {ComplainerProfile,Complaint} to BKSV's web form

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/skypies/util/date"
	
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/config"
)

//https://viewpoint.emsbk.com/<sitename>?response=json
const bksvHost = "viewpoint.emsbk.com"
const bksvPath = "/sfo5"

// {{{ PopulateForm

func PopulateForm(c types.Complaint, submitkey string) url.Values {
	first,last := c.Profile.SplitName()
	addr := c.Profile.GetStructuredAddress()
	if c.Activity == "" { c.Activity = "Loud noise" }

	vals := url.Values{
		"response":         {"json"},
		"contactmethod":    {"App"},
		"apiKey":           {config.Get("bksv.apiKey")},
		
		"caller_code":      {c.Profile.CallerCode},
		"name":             {first},
		"surname":          {last},
		"address1":         {addr.Number + " " + addr.Street},
		"address2":         {""},
		"zipcode":          {addr.Zip},
		"city":             {addr.City},
		"state":            {addr.State},
		"email":            {c.Profile.EmailAddress},

		"airports":         {"KSFO"},  // KOAK, KSJC, KSAN
		"month":            {date.InPdt(c.Timestamp).Format("1")},
		"day":              {date.InPdt(c.Timestamp).Format("2")},
		"year":             {date.InPdt(c.Timestamp).Format("2006")},
		"hour":             {date.InPdt(c.Timestamp).Format("15")},
		"min":              {date.InPdt(c.Timestamp).Format("4")},
		
		"aircraftcategory": {"J"},
		"eventtype":        {"Loud noise"}, // perhaps map c.Activity to something ?
		"comments":         {c.Description},
		"responserequired": {"N"},
		"enquirytype":      {"C"},

		"submit":           {"Submit complaint"},
		//"submitkey":        {submitkey},

		"nowebtrak": {"1"},
		"defaulttime": {"0"},
		"webtraklinkback": {""},
		"title": {""},
		"homephone": {""},
		"workphone": {""},
		"cellphone": {""},

		"browser_name":     {c.Browser.Name},
		"browser_version":  {c.Browser.Version},
		"browser_vendor":   {c.Browser.Vendor},
		"browser_uuid":     {c.Browser.UUID},
		"browser_platform": {c.Browser.Platform},
	}

	if c.AircraftOverhead.FlightNumber != "" {
		vals.Add("acid", c.AircraftOverhead.Callsign)
		vals.Add("aacode", c.AircraftOverhead.Id2)
		vals.Add("tailnumber", c.AircraftOverhead.Registration)
		vals.Add("aircrafttype", c.AircraftOverhead.EquipType)
			
		//vals.Add("adflag", "??") // Operation type (A, D or O for Arr, Dept or Overflight)
		//vals.Add("beacon", "??") // Squawk SSR code (eg 2100)
	}

	return vals
}

// }}}
// {{{ PostComplaint

// https://complaints-staging.bksv.com/sfo2?json=1&resp=json
// {"result":"1",
//  "title":"Complaint Received",
//  "body":"Thank you. We have received your complaint."}

func PostComplaint(client *http.Client, c types.Complaint) (*types.Submission, error) {
	// Initialize a new submission object, inheriting from previous
	s := types.Submission{
		Attempts:  c.Submission.Attempts + 1,
		Log:       c.Submission.Log+fmt.Sprintf("\n--------=={ PostComplaint @ %s }==-\n", time.Now()),
		Key:       c.Submission.Key, // We're now keyless, should prob strip this out
		T:         time.Now().UTC(),
		Outcome:   types.SubmissionFailed, // Be pessimistic right up until the end
	}

	// We used to have to fetch a unique key (which lived in the form),
	// that we'd need to submit with the rest of the complaint; that
	// prevented dupes on their end. But the new version skips that
	// requirement for API based submissions like ours, so we're
	// keyless now.
	s.Log += fmt.Sprintf("----{ time: %s }----\n  --{ keyless submission }--\n", s.T)

	vals := PopulateForm(c, "")
	s.Log += "Submitting these vals:-\n"
	for k,v := range vals { s.Log += fmt.Sprintf(" * %-20.20s: %v\n", k, v) }
	s.Log += "\n"

	resp,err := client.PostForm("https://"+bksvHost+bksvPath, vals)
	s.D = time.Since(s.T)
	if err != nil {
		if strings.Contains(err.Error(), "DEADLINE_EXCEEDED") {
			s.Outcome = types.SubmissionTimeout
		}
		s.Log += fmt.Sprintf("ComplaintPOST: Posting error (dur=%s): %v\n", s.D, err)
		return &s,err
	}

	defer resp.Body.Close()
	body,_ := ioutil.ReadAll(resp.Body)
	s.Log += fmt.Sprintf("ComplaintPOST: HTTP response '%s'\n", resp.Status)
	s.Response = []byte(body)
	if resp.StatusCode >= 400 {
		s.Log += fmt.Sprintf("ComplaintPOST: HTTP Body:-\n%s\n--\n", body)
		return &s,fmt.Errorf("ComplaintPOST: HTTP err %s", resp.Status)
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(body), &jsonMap); err != nil {
		s.Log += fmt.Sprintf("ComplaintPOST: JSON unmarshal '%v'\nBody:-\n%s\n--\n", err, body)
		return &s,fmt.Errorf("ComplaintPOST: JSON unmarshal %v", err)
		/* A few times, the remote site failed to send JSON responses, and sent HTML instead. This
     * will work in that case.
			if !regexp.MustCompile(`(?i:received your complaint)`).MatchString(string(body)) {
				debug += fmt.Sprintf("BKSV body ...\n%s\n------\n", string(body))
				return debug,fmt.Errorf("Returned response did not say 'received your complaint'")
			} else {
				debug += "Success !\n"+string(body)
			}
     */			
	}

	indentedBytes,_ := json.MarshalIndent(jsonMap, "", "  ")
	s.Log += "\n-- JsonMap:-\n"+string(indentedBytes)+"\n--\n"

/*
-- JsonMap:-
{
  "body": "Thank you. We have received your complaint.",
  "details": {
    "address_id": 1967,
    "complaint_id": 1.120211e+06,
    "device_id": 1,
    "person_id": 2083
  },
  "result": "1",
  "title": "Complaint Received"
}
--
*/
	
	v := jsonMap["result"];
	if v == nil {
		s.Log += fmt.Sprintf("ComplaintPOST: json no 'result'\n")
		return &s,fmt.Errorf("ComplaintPOST: jsonmap had no 'result'")
	}

	result := v.(string)
	if result != "1" {
		s.Outcome = types.SubmissionRejected
		s.Log += fmt.Sprintf("Json result not '1'\n")
		return &s,fmt.Errorf("ComplaintPOST: result='%s'", result)
	}

	s.Log += "Json Success !\n"
	s.Outcome = types.SubmissionAccepted

	// Extract the foreign key for this complaint
	detailsMap := jsonMap["details"].(map[string]interface{})
	s.Key = fmt.Sprintf("%.0f", detailsMap["complaint_id"].(float64))

	return &s,nil
}

// }}}

// {{{ Notes

/* These POST fields were sent by browser, for happy success

nowebtrak:1
submitkey:4aef9c8831919524ec35ae8af8ff25ba
defaulttime:0
webtraklinkback:
title:
name:Adam
surname:Worrall
address1:1 Some Drive
address2:
city:Scotts Valley
state:CA
homephone:
workphone:
cellphone:
email:adam-foosite@worrall.cc
airports:KSFO
month:10
day:2
year:2015
hour:20
min:16
aircrafttype:Jet
eventtype:Loud noise
comments:Blah
responserequired:N
enquirytype:C
submit:Submit complaint

*/


/* BKSV:

You can call it this way

 https://complaints-staging.bksv.com/sfo2?json=1

to get a json object of all the form field definitions we accept. That
will tell you what fields we require to be set and also gives you
handy things like the set of allowed disturbance types, event types
etc. NB: I haven't yet configured it to match the SFO system values
for these but that is a simple config change I can do as soon as I
have the information.

{
    "airports": "{ \"KSFO\": \"San Francisco International Airport (SFO)\" , \"KSAN\": \"San Diego International Airport (SAN)\", \"KOAK\": \"Oakland International Airport (OAK)\", \"KSJC\": \"Mineta San José International Airport (SJC)\" }",
    "locale": "en_AU",
    "displayAreaCodes": "0",
    "submitKey": "797eaa0e960b5e8848ce6785950dfd3c",

    "hours": [
        "12 AM",
        "1 AM",
        "2 AM",
        "3 AM",
        "4 AM",
        "5 AM",
        "6 AM",
        "7 AM",
        "8 AM",
        "9 AM",
        "10 AM",
        "11 AM",
        "12 PM",
        "1 PM",
        "2 PM",
        "3 PM",
        "4 PM",
        "5 PM",
        "6 PM",
        "7 PM",
        "8 PM",
        "9 PM",
        "10 PM",
        "11 PM"
    ],

    "atLeastOneContact": true,
    "field_defs": {
        "address2": {
            "maxlength": 124,
            "required": false,
            "scope": "profile",
            "type": "text",
            "label": "Address (line 2)"
        },

        "webtrak": {
            "maxlength": 0,
            "required": false,
            "scope": "ignore",
            "type": "ignore",
            "label": "Information from WebTrak"
        },
        "email": {
            "maxlength": 64,
            "required": false,
            "scope": "profile",
            "type": "email",
            "label": "Email"
        },

        "text2": {
            "maxlength": 0,
            "required": false,
            "scope": "about",
            "type": "content",
            "label": ""
        },
        "state": {
            "maxlength": 100,
            "required": true,
            "scope": "profile",
            "type": "list",
            "label": "State"
        },

        "responserequired": {
            "maxlength": 0,
            "required": true,
            "scope": "profile",
            "type": "boolean",
            "label": "Would you like to be contacted by one of our staff?"
        },
        "enquirytype": {
            "maxlength": 0,
            "required": true,
            "scope": "complaint",
            "type": "list",
            "label": "Enquiry type"
        },

        "time": {
            "maxlength": 0,
            "required": true,
            "scope": "complaint",
            "type": "datetime",
            "label": "Disturbance time"
        },
        "workphone": {
            "maxlength": 62,
            "required": false,
            "scope": "profile",
            "type": "tel",
            "label": "Work phone"
        },

        "airports": {
            "maxlength": 0,
            "required": true,
            "scope": "complaint",
            "type": "list",
            "label": "Airport"
        },
        "contact": {
            "maxlength": 0,
            "required": false,
            "scope": "ignore",
            "type": "ignore",
            "label": "Contact number"
        },

        "date": {
            "maxlength": 0,
            "required": true,
            "scope": "complaint",
            "type": "datetime",
            "label": "Disturbance date"
        },
        "text1": {
            "maxlength": 0,
            "required": false,
            "scope": "about",
            "type": "content",
            "label": ""
        },
        "eventtype": {
            "maxlength": 0,
            "required": false,
            "scope": "complaint",
            "type": "list",
            "label": "Disturbance type"
        },

        "name": {
            "maxlength": 62,
            "required": true,
            "scope": "profile",
            "type": "text",
            "label": "First name"
        },
        "city": {
            "maxlength": 46,
            "required": true,
            "scope": "profile",
            "type": "text",
            "label": "City"
        },
        "address1": {
            "maxlength": 124,
            "required": true,
            "scope": "profile",
            "type": "text",
            "label": "Address"
        },

        "cellphone": {
            "maxlength": 62,
            "required": false,
            "scope": "profile",
            "type": "tel",
            "label": "Mobile phone"
        },
        "aircrafttype": {
            "maxlength": 0,
            "required": false,
            "scope": "complaint",
            "type": "list",
            "label": "Aircraft type"
        },
        "comments": {
            "maxlength": 10000,
            "required": false,
            "scope": "complaint",
            "type": "textarea",
            "label": "Please give details"
        },

        "title": {
            "maxlength": 30,
            "required": false,
            "scope": "profile",
            "type": "list",
            "label": "Title"
        },
        "surname": {
            "maxlength": 62,
            "required": true,
            "scope": "profile",
            "type": "text",
            "label": "Last name"
        },
        "homephone": {
            "maxlength": 62,
            "required": false,
            "scope": "profile",
            "type": "tel",
            "label": "Home phone"
        }
    },

    "years": {
        "2015": "2015",
        "2014": 2014
    },
    "dateFormat": [
        "month",
        "day",
        "year"
    ],

    "strings": {
        "months/short/5": "Jun",
        "labels/month": "Month",
        "complaintsform/lists/acTypes": "Jet,Propeller,Helicopter,Various,Unknown",
        "months/short/3": "Apr",
        "complaintsform/lists/activity_types": "Indoors,Outdoors,Watching TV,Sleeping,Working,Other",
        "labels/hour": "Hour",
        "labels/year": "Year",
        "months/short/4": "May",
        "months/short/9": "Oct",
        "months/short/2": "Mar",
        "complaintsform/app/complaintReceived": "Complaint received!",
        "complaintsform/lists/event_types": "Loud noise,Overflight,Low flying,Early turn,Go-around,Too frequent,Helicopter operations,Engine run-up,Ground noise,Other",
        "complaintsform/blocks/submitComplaint": "Submit complaint",
        "months/short/7": "Aug",
        "complaintsform/blocks/pleaseFillIn": "Please fill in",
        "timeOfDay/1": "PM",
        "complaintsform/blocks/tooShort": "Value is too short",
        "complaintsform/lists/acModes_internal": "",
        "complaintsform/blocks/required": "(required)",
        "months/short/8": "Sep",
        "complaintsform/lists/acModes": "Arrival,Departure,Overflight,Unknown",
        "labels/minute": "Min",
        "timeOfDay/0": "AM",
        "months/short/6": "Jul",
        "complaintsform/lists/acTypes_internal": "",
        "labels/yes": "Yes",
        "months/short/10": "Nov",
        "months/short/1": "Feb",
        "complaintsform/lists/titles": "Mr,Mrs,Miss,Ms,Dr",
        "complaintsform/lists/contact_method": "Letter,Email,Telephone",
        "labels/no": "No",
        "complaintsform/blocks/errors": "There are some problems. Please correct the mistakes and submit the form again.",
        "labels/day": "Day",
        "months/short/0": "Jan",
        "lists/state": "CA,AZ",
        "months/short/11": "Dec"
    },

    "fields": [
        "text1",
        "title",
        "name",
        "surname",
        "address1",
        "address2",
        "city",
        "state",
        "contact",
        "airports",
        "text2",
        "date",
        "time",
        "webtrak",
        "aircrafttype",
        "eventtype",
        "comments",
        "responserequired",
        "enquirytype",
        "homephone",
        "workphone",
        "cellphone",
        "email"
    ]
}

*/

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
