package templates

import (
	"embed"
	"html/template"
)

//go:embed *.html
var templateFS embed.FS

// funcs are helpers available in every template. inc turns a 0-based
// range index into a 1-based position (used for result rank).
var funcs = template.FuncMap{"inc": func(i int) int { return i + 1 }}

var layoutBase = template.Must(template.New("layout.html").Funcs(funcs).ParseFS(templateFS, "layout.html"))

// Page returnerer layout.html kombineret med præcis én sides "body"-indhold.
func Page(name string) *template.Template {
	clone := template.Must(layoutBase.Clone())
	return template.Must(clone.ParseFS(templateFS, name))
}
