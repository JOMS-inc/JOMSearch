package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Mock tests: the weather code calls the external Open-Meteo API. Here we
// replace it with a fake server, so the tests are fast, deterministic and
// don't fail when the real service is down or the weather changes.

// fakeOpenMeteo starts a fake Open-Meteo that answers with status and body,
// points openMeteoURL at it for the rest of the test, and records the query
// string of the last request it got.
func fakeOpenMeteo(t *testing.T, status int, body string) *url.Values {
	t.Helper()
	var lastQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.Query()
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	original := openMeteoURL
	openMeteoURL = srv.URL
	t.Cleanup(func() { openMeteoURL = original })
	return &lastQuery
}

const sunnyAarhus = `{
	"latitude": 56.16, "longitude": 10.2,
	"current": {"time": "2026-11-02T13:15", "temperature_2m": 8.4, "wind_speed_10m": 14.2, "weather_code": 0}
}`

func TestWeatherAPIMapsResponse(t *testing.T) {
	query := fakeOpenMeteo(t, http.StatusOK, sunnyAarhus)

	rec := httptest.NewRecorder()
	Weather(rec, httptest.NewRequest(http.MethodGet, "/api/weather?lat=56.1629&lon=10.2039", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got WeatherResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	want := WeatherResponse{Latitude: 56.16, Longitude: 10.2, Time: "2026-11-02T13:15", Temperature: 8.4, WindSpeed: 14.2, WeatherCode: 0}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Our coordinates were passed on to the upstream API.
	if (*query).Get("latitude") != "56.1629" || (*query).Get("longitude") != "10.2039" {
		t.Errorf("upstream got lat=%s lon=%s", (*query).Get("latitude"), (*query).Get("longitude"))
	}
}

func TestWeatherAPIRejectsBadCoordinates(t *testing.T) {
	fakeOpenMeteo(t, http.StatusOK, sunnyAarhus)

	for _, q := range []string{"lat=abc", "lat=91", "lat=-91", "lon=181", "lon=xyz"} {
		rec := httptest.NewRecorder()
		Weather(rec, httptest.NewRequest(http.MethodGet, "/api/weather?"+q, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rec.Code)
		}
	}
}

func TestWeatherAPIUpstreamFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"upstream 500", http.StatusInternalServerError, "boom"},
		{"upstream sends garbage", http.StatusOK, "not json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeOpenMeteo(t, tt.status, tt.body)
			rec := httptest.NewRecorder()
			Weather(rec, httptest.NewRequest(http.MethodGet, "/api/weather", nil))
			if rec.Code != http.StatusBadGateway {
				t.Errorf("status = %d, want 502", rec.Code)
			}
		})
	}
}

func TestNewWeatherPageData(t *testing.T) {
	query := fakeOpenMeteo(t, http.StatusOK, sunnyAarhus)

	data := NewWeatherPageData(context.Background(), "Aarhus")
	if data.Selected != "Aarhus" || (*query).Get("latitude") != "56.1629" {
		t.Errorf("Aarhus not used: selected=%q, upstream lat=%s", data.Selected, (*query).Get("latitude"))
	}
	if data.Description != "Klar himmel" {
		t.Errorf("Description = %q, want %q", data.Description, "Klar himmel")
	}
	if data.Updated != "02-11-2026 13:15" {
		t.Errorf("Updated = %q, want %q", data.Updated, "02-11-2026 13:15")
	}

	// Unknown city falls back to København.
	if data := NewWeatherPageData(context.Background(), "Atlantis"); data.Selected != "København" {
		t.Errorf("unknown city: Selected = %q, want København", data.Selected)
	}
}

func TestNewWeatherPageDataUnknownCode(t *testing.T) {
	fakeOpenMeteo(t, http.StatusOK, strings.Replace(sunnyAarhus, `"weather_code": 0`, `"weather_code": 12345`, 1))
	if data := NewWeatherPageData(context.Background(), ""); data.Description != "Ukendt vejr" {
		t.Errorf("Description = %q, want %q", data.Description, "Ukendt vejr")
	}
}

func TestWeatherPage(t *testing.T) {
	app := newTestApp(t)

	t.Run("shows the weather", func(t *testing.T) {
		fakeOpenMeteo(t, http.StatusOK, sunnyAarhus)
		status, body := app.get(t, "/weather?city=Aarhus")
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		for _, want := range []string{"Klar himmel", "8.4 °C", "14.2 km/t"} {
			if !strings.Contains(body, want) {
				t.Errorf("page does not contain %q", want)
			}
		}
	})

	t.Run("shows an error when the API is down", func(t *testing.T) {
		fakeOpenMeteo(t, http.StatusServiceUnavailable, "")
		status, body := app.get(t, "/weather")
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if !strings.Contains(body, "Kunne ikke hente vejret") {
			t.Error("page does not show the error message")
		}
	})
}
