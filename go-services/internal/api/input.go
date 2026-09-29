package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// parseInput fills r.Form from either a JSON object body or a regular
// form-encoded body, so handlers can keep using r.FormValue for both.
func parseInput(r *http.Request) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return r.ParseForm()
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	form := url.Values{}
	for k, v := range body {
		if s, ok := v.(string); ok {
			form.Set(k, s)
		}
	}
	r.Form = form
	return nil
}
