package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Unit tests for parseInput: the same handler code must accept both
// HTML form posts and JSON bodies.

func newPost(body, contentType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	return r
}

func TestParseInputForm(t *testing.T) {
	r := newPost("username=alice&password=pw", "application/x-www-form-urlencoded")
	if err := parseInput(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.FormValue("username"); got != "alice" {
		t.Errorf("username = %q, want %q", got, "alice")
	}
	if got := r.FormValue("password"); got != "pw" {
		t.Errorf("password = %q, want %q", got, "pw")
	}
}

func TestParseInputJSON(t *testing.T) {
	r := newPost(`{"username":"alice","password":"pw"}`, "application/json; charset=utf-8")
	if err := parseInput(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.FormValue("username"); got != "alice" {
		t.Errorf("username = %q, want %q", got, "alice")
	}
}

func TestParseInputJSONIgnoresNonStrings(t *testing.T) {
	r := newPost(`{"username":"alice","age":42,"admin":true,"tags":["x"]}`, "application/json")
	if err := parseInput(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, key := range []string{"age", "admin", "tags"} {
		if got := r.FormValue(key); got != "" {
			t.Errorf("%s = %q, want it dropped", key, got)
		}
	}
}

func TestParseInputInvalidJSON(t *testing.T) {
	for _, body := range []string{`{"username":`, `["a","b"]`, `not json`} {
		r := newPost(body, "application/json")
		if err := parseInput(r); err == nil {
			t.Errorf("parseInput(%q) returned nil error, want an error", body)
		}
	}
}

// FuzzParseInput feeds random bodies to parseInput. The seed corpus below runs
// on every `go test`; run the real fuzzer with:
//
//	go test -fuzz=FuzzParseInput -fuzztime=30s ./internal/api
//
// It checks that parseInput never panics on any body, and that on success
// r.Form is in a usable state (no keys without values).
func FuzzParseInput(f *testing.F) {
	f.Add(`{"username":"alice","password":"pw"}`, true)
	f.Add(`username=alice&password=pw`, false)
	f.Add(`{"a":{"nested":1}}`, true)
	f.Add(`%zz=1`, false)
	f.Add(``, true)

	f.Fuzz(func(t *testing.T, body string, isJSON bool) {
		contentType := "application/x-www-form-urlencoded"
		if isJSON {
			contentType = "application/json"
		}
		r := newPost(body, contentType)
		if err := parseInput(r); err != nil {
			return // rejecting bad input is fine; panicking is not
		}
		for key, values := range r.Form {
			if len(values) == 0 {
				t.Errorf("key %q has no values", key)
			}
		}
	})
}
