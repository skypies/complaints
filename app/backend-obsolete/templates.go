package backend

import(
	"errors"
	"html/template"
	"regexp"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
)

var templates *template.Template
var HackTemplates *template.Template  // FIXME: remove this

func init() {
	// Templates are kinda messy.
	// The functions to parse them live in the UI library.
	// The "templates" dir lives under the appengine app's main dir; to reuse templates
	// from other places, we symlink them underneath this.
	// For go111, appengine uses the module root, which is the root of the git repo; so
	// the relative dirname for templates is relative to the root of the git repo.
	templates = widget.ParseRecursive(template.New("").Funcs(GetFuncMap()), "backend/web/templates")
	HackTemplates = templates
}

func GetFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": templateAdd,
		"km2feet": templateKM2Feet,
		"spacify": templateSpacifyFlightNumber,
		"dict": templateDict,
		"dict4select": templateDict4select,
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
func templateDict4select(name, dflt string, vals [][]string) map[string]interface{} {
	return map[string]interface{}{
		"Name": name,
		"Default": dflt,
		"Vals": vals,
	}
}
