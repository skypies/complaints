package main

import(
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"time"

	"github.com/skypies/flightdb/ui"
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	appengineds "github.com/skypies/util/ae/ds"
)

var templates *template.Template

func init() {
	// Load up our parsing functions, and the ones for flightdb templates
	base := template.New("").Funcs(GetFuncMap()).Funcs(ui.TemplateFuncMap())
	templates = widget.ParseRecursive(base, "templates")

	// Kind of a hack; we can only trigger this after templates has been initialized.
	http.HandleFunc("/map", widget.WithCtxTmpl(appengineds.CtxMakerFunc, templates, ui.MapHandler))
}

func GetFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": templateAdd,
		"km2feet": templateKM2Feet,
		"spacify": templateSpacifyFlightNumber,
		"dict": templateDict,
		"selectdict": templateSelectDict,
		"formatPdt": templateFormatPdt,
	}
}

func templateAdd(a int, b int) int { return a + b }
func templateKM2Feet(x float64) float64 { return x * 3280.84 }
func templateSpacifyFlightNumber(s string) string {
	s2 := regexp.MustCompile("^r:(.+)$").ReplaceAllString(s, "Registration:$1")
	s3 := regexp.MustCompile("^(..)(\\d\\d\\d)$").ReplaceAllString(s2, "$1 $2")
	return regexp.MustCompile("^(..)(\\d\\d)$").ReplaceAllString(s3, "$1  $2")
}
func templateDict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 { return nil, errors.New("invalid dict call")	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i+=2 {
		key, ok := values[i].(string)
		if !ok { return nil, errors.New("dict keys must be strings") }
		dict[key] = values[i+1]
	}
	return dict, nil
}
func templateFormatPdt(t time.Time, format string) string {
	return date.InPdt(t).Format(format)
}

func templateSelectDict(name, dflt string, vals interface{}) map[string]interface{} {
	return map[string]interface{}{
		"Name": name,
		"Default": dflt,
		"Vals": vals,
	}
}
