package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/testdb"
)

// Integration tests: the real router, handlers, templates, sessions and a real
// (temporary, seeded) SQLite database, exercised over HTTP like a browser would.
//
// These tests don't use t.Parallel() because the session store and dbConn
// (see auth.go) are package-level globals shared by all handlers.

type testApp struct {
	srv    *httptest.Server
	db     *sql.DB
	client *http.Client
}

// newTestApp wires the app up the same way cmd/search-api/main.go does.
func newTestApp(t *testing.T) *testApp {
	t.Helper()
	conn := testdb.New(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	RegisterRoutes(mux, conn)
	RegisterSearchRoutes(mux, conn)
	SetDB(conn)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &testApp{srv: srv, db: conn, client: &http.Client{Jar: jar}}
}

// get fetches path and returns the status code and body.
func (a *testApp) get(t *testing.T, path string) (int, string) {
	t.Helper()
	resp, err := a.client.Get(a.srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// postForm submits form to path (following the redirect, like a browser).
func (a *testApp) postForm(t *testing.T, path string, form url.Values) (int, string) {
	t.Helper()
	resp, err := a.client.PostForm(a.srv.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestPagesRespond(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		path     string
		contains string
	}{
		{"/health", "ok"},
		{"/", `id="search-input"`},
		{"/search", `id="search-input"`},
		{"/about", "Our mission"},
		{"/login", "Log In"},
		{"/register", "Sign Up"},
		{"/static/style.css", "div.page"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			status, body := app.get(t, tt.path)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			if !strings.Contains(body, tt.contains) {
				t.Errorf("body does not contain %q", tt.contains)
			}
		})
	}

	if status, _ := app.get(t, "/does-not-exist"); status != http.StatusNotFound {
		t.Errorf("unknown path: status = %d, want 404", status)
	}
}

func TestSearchAPI(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name      string
		query     string
		wantTitle []string
	}{
		{"english matches", "q=programming", []string{"Go (programming language)", "SQLite"}},
		{"defaults to english", "q=hovedstad", nil},
		{"danish with language=da", "q=hovedstad&language=da", []string{"København"}},
		{"no match", "q=zzzznotfound", nil},
		{"empty query returns nothing", "q=", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := app.get(t, "/api/search?"+tt.query)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			var got SearchResponse
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("invalid JSON %q: %v", body, err)
			}
			if len(got.SearchResults) != len(tt.wantTitle) {
				t.Fatalf("got %d results, want %d: %+v", len(got.SearchResults), len(tt.wantTitle), got.SearchResults)
			}
			for i, title := range tt.wantTitle {
				if got.SearchResults[i].Title != title {
					t.Errorf("result %d title = %q, want %q", i, got.SearchResults[i].Title, title)
				}
			}
		})
	}
}

func TestSearchPageShowsResults(t *testing.T) {
	app := newTestApp(t)
	status, body := app.get(t, "/?q=SQLite")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, `href="https://en.wikipedia.org/wiki/SQLite"`) {
		t.Error("search page does not link to the SQLite result")
	}
}

func TestRegisterLoginLogoutFlow(t *testing.T) {
	app := newTestApp(t)

	// Register: should create the user, log in and land on the search page.
	status, body := app.postForm(t, "/api/register", url.Values{
		"username": {"alice"}, "email": {"alice@example.com"},
		"password": {"pw123456"}, "password2": {"pw123456"},
	})
	if status != http.StatusOK || !strings.Contains(body, "Log out [alice]") {
		t.Fatalf("after register: status %d, logged-in nav missing", status)
	}

	// The password is stored as a bcrypt hash, never in plain text.
	var stored string
	if err := app.db.QueryRow(`SELECT password FROM users WHERE username = 'alice'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$2") {
		t.Errorf("stored password %q is not a bcrypt hash", stored)
	}

	// Logout: the session is cleared.
	if _, body := app.get(t, "/logout"); strings.Contains(body, "Log out") {
		t.Fatal("still logged in after logout")
	}

	// Log back in with the same credentials.
	status, body = app.postForm(t, "/login", url.Values{"username": {"alice"}, "password": {"pw123456"}})
	if status != http.StatusOK || !strings.Contains(body, "Log out [alice]") {
		t.Fatalf("after login: status %d, logged-in nav missing", status)
	}
}

func TestLoginWithJSON(t *testing.T) {
	app := newTestApp(t)
	app.postForm(t, "/api/register", url.Values{
		"username": {"bob"}, "email": {"bob@example.com"},
		"password": {"pw"}, "password2": {"pw"},
	})
	app.get(t, "/logout")

	resp, err := app.client.Post(app.srv.URL+"/api/login", "application/json",
		strings.NewReader(`{"username":"bob","password":"pw"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Log out [bob]") {
		t.Fatalf("JSON login: status %d, logged-in nav missing", resp.StatusCode)
	}
}

func TestRegisterErrors(t *testing.T) {
	app := newTestApp(t)
	valid := url.Values{
		"username": {"carol"}, "email": {"carol@example.com"},
		"password": {"pw"}, "password2": {"pw"},
	}
	app.postForm(t, "/api/register", valid)
	app.get(t, "/logout")

	tests := []struct {
		name       string
		form       url.Values
		wantStatus int
		wantMsg    string
	}{
		{"missing fields", url.Values{"username": {"dave"}}, http.StatusUnprocessableEntity, "All fields are required."},
		{"passwords differ", url.Values{
			"username": {"dave"}, "email": {"dave@example.com"},
			"password": {"a"}, "password2": {"b"},
		}, http.StatusUnprocessableEntity, "Passwords do not match."},
		{"duplicate user", valid, http.StatusConflict, "Username or email already taken."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := app.postForm(t, "/api/register", tt.form)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if !strings.Contains(body, tt.wantMsg) {
				t.Errorf("body does not contain %q", tt.wantMsg)
			}
		})
	}
}

func TestLoginErrors(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name string
		form url.Values
		msg  string
	}{
		{"missing password", url.Values{"username": {"admin"}}, "Username and password are required."},
		{"wrong password", url.Values{"username": {"admin"}, "password": {"nope"}}, "Invalid username or password."},
		{"unknown user", url.Values{"username": {"ghost"}, "password": {"x"}}, "Invalid username or password."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := app.postForm(t, "/login", tt.form)
			if status != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", status)
			}
			if !strings.Contains(body, tt.msg) {
				t.Errorf("body does not contain %q", tt.msg)
			}
		})
	}
}

func TestLegacyMD5LoginUpgradesHash(t *testing.T) {
	app := newTestApp(t)

	// The seeded admin user (schema/schema.sql) has an MD5 hash of "password".
	status, body := app.postForm(t, "/login", url.Values{"username": {"admin"}, "password": {"password"}})
	if status != http.StatusOK || !strings.Contains(body, "Log out [admin]") {
		t.Fatalf("legacy login failed: status %d", status)
	}

	var stored string
	if err := app.db.QueryRow(`SELECT password FROM users WHERE username = 'admin'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$2") {
		t.Errorf("admin hash was not upgraded to bcrypt, got %q", stored)
	}
}
