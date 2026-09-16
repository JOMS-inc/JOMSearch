package templates

import (
	"embed"
	"html/template"
)

//go:embed *.html
var templateFS embed.FS

var layoutBase = template.Must(template.ParseFS(templateFS, "layout.html"))

// Page returnerer layout.html kombineret med præcis én sides "body"-indhold.
func Page(name string) *template.Template {
	clone := template.Must(layoutBase.Clone())
	return template.Must(clone.ParseFS(templateFS, name))
}