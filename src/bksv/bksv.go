package bksv

// Package for posting a {ComplainerProfile,Complaint} to BKSV's web form

import (
	//"bytes"
	//"net/http/httputil"
	
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"

	"github.com/skypies/date"
	
	"complaintdb/types"
)

const bksvHost = "complaints-staging.bksv.com"
const bksvPath = "/sfo2"

// {{{ GetSubmitkey

// Should really get this from the ?json=1 version of the URL and extract it.

// <input type=hidden name=submitkey value="c0a9e3dfa16a586e2ddfd2dfe9bcc002">
func GetSubmitkey(client *http.Client) (string, string, error) {
	debug := ""
	
	if resp,err := client.Get("https://"+bksvHost+bksvPath+"?json=1"); err != nil {
		return debug,"",err

	} else {
		if false {
			defer resp.Body.Close()
			body,_ := ioutil.ReadAll(resp.Body)		
			out := regexp.MustCompile(`"submitKey":"([^"]+)"`).FindStringSubmatch(string(body))
			if out == nil {
				debug += fmt.Sprintf("GetSubmitKey: regexp failued lookup: %v\n")
				return debug,"",fmt.Errorf("GetSubmitKey/regexp/none found")
			}
			return debug, out[1], nil
			
		} else {
			var jsonMap map[string]interface{}

			err = json.NewDecoder(resp.Body).Decode(&jsonMap)
			if v := jsonMap["submitKey"]; v == nil {
				debug += fmt.Sprintf("GetSubmitKey: Jsonresponse was empty: %v\n", jsonMap)
				return debug,"",fmt.Errorf("GetSubmitKey/NoJsonField")
				
			} else if submitkey := v.(string); submitkey == "" {
				return debug,"",fmt.Errorf("GetSubmitKey/init response did not have a submitkey")

			} else {
				return debug,submitkey,nil
			}
		}
	}
}

// }}}
// {{{ PostComplaint

// https://complaints-staging.bksv.com/sfo2?json=1&resp=json
// {"result":"1",
//  "title":"Complaint Received",
//  "body":"Thank you. We have received your complaint."}

func PostComplaint(client *http.Client, p types.ComplainerProfile, c types.Complaint) (string,error) {
	first,last := p.SplitName()
	addr := p.GetStructuredAddress()
	if c.Activity == "" { c.Activity = "Loud noise" }

	debug, submitkey, err := GetSubmitkey(client)
	if err != nil { return debug,err }
	debug += fmt.Sprintf("We got submitkey=%s\n", submitkey)
	
	// {{{ Populate form

	vals := url.Values{
		"response":         {"json"},

		"contactmethod":    {"App"},
		"app_key":          {"TUC8uDJMooVMvf7hew93nhUGcWgw"},
		
		"caller_code":      {p.CallerCode},
		"name":             {first},
		"surname":          {last},
		"address1":         {addr.Street},
		"address2":         {""},
		"zipcode":          {addr.Zip},
		"city":             {addr.City},
		"state":            {addr.State},
		"email":            {p.EmailAddress},

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
		"submitkey":        {submitkey},

		"nowebtrak": {"1"},
		"defaulttime": {"0"},
		"webtraklinkback": {""},
		"title": {""},
		"homephone": {""},
		"workphone": {""},
		"cellphone": {""},
	}

	if c.AircraftOverhead.FlightNumber != "" {
		vals.Add("acid", c.AircraftOverhead.Callsign)
		vals.Add("aacode", c.AircraftOverhead.Id2)
		vals.Add("tailnumber", c.AircraftOverhead.Registration)
		vals.Add("aircrafttype", c.AircraftOverhead.EquipType)
			
		//vals.Add("adflag", "??") // Operation type (A, D or O for Arr, Dept or Overflight)
		//vals.Add("beacon", "??") // SSR code (eg 210)
	}

	// }}}
	
	debug += "Submitting these vals:-\n"
	for k,v := range vals { debug += fmt.Sprintf(" * %-20.20s: %v\n", k, v) }
	
	if resp,err := client.PostForm("https://"+bksvHost+bksvPath, vals); err != nil {
		return debug,err

	} else {

		defer resp.Body.Close()
		body,_ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			debug += fmt.Sprintf("ComplaintPOST: HTTP err '%s'\nBody:-\n%s\n--\n", resp.Status, body)
			return debug,fmt.Errorf("ComplaintPOST: HTTP err %s", resp.Status)
		}

		var jsonMap map[string]interface{}
		if err := json.Unmarshal([]byte(body), &jsonMap); err != nil {
			debug += fmt.Sprintf("ComplaintPOST: JSON unmarshal '%v'\nBody:-\n%s\n--\n", err, body)
			return debug,fmt.Errorf("ComplaintPOST: JSON unmarshal %v", err)

			/* Fall back ?
			if !regexp.MustCompile(`(?i:received your complaint)`).MatchString(string(body)) {
				debug += fmt.Sprintf("BKSV body ...\n%s\n------\n", string(body))
				return debug,fmt.Errorf("Returned response did not say 'received your complaint'")
			} else {
				debug += "Success !\n"+string(body)
			}
      */
			
		} else if v := jsonMap["result"]; v == nil {
			return debug,fmt.Errorf("ComplaintPOST: jsonmap had no 'result'.\nBody:-\n%s\n--\n", body)

		} else {
			result := v.(string)
			if result == "1" {
					debug += "Json Success !\n"
				} else {
					debug += fmt.Sprintf("Json result not '1':-\n%#v\n--\n", jsonMap)
					return debug,fmt.Errorf("ComplaintPOST: result='%s'", result)
				}
			}
	}

	return debug,nil
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


/*

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
