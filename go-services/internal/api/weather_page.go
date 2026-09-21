package api

import (
	"context"
	"net/http"
	"time"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

type City struct {
	Name string
	Lat  float64
	Lon  float64
}

var Cities = []City{
	{"København", 55.6761, 12.5683},
	{"Aarhus", 56.1629, 10.2039},
	{"Odense", 55.4038, 10.4024},
	{"Aalborg", 57.0488, 9.9217},
}

// WMO weather codes -> dansk tekst
var weatherDescriptions = map[int]string{
	0: "Klar himmel", 1: "Mest klar", 2: "Delvist skyet", 3: "Overskyet",
	45: "Tåge", 48: "Rimtåge",
	51: "Let støvregn", 53: "Støvregn", 55: "Kraftig støvregn",
	56: "Isslag (støvregn)", 57: "Kraftigt isslag (støvregn)",
	61: "Let regn", 63: "Regn", 65: "Kraftig regn",
	66: "Isslag (regn)", 67: "Kraftigt isslag (regn)",
	71: "Let snefald", 73: "Snefald", 75: "Kraftigt snefald", 77: "Snekorn",
	80: "Lette byger", 81: "Regnbyger", 82: "Voldsomme byger",
	85: "Snebyger", 86: "Kraftige snebyger",
	95: "Tordenvejr", 96: "Torden med hagl", 99: "Kraftig torden med hagl",
}

// Data der sendes til weather.html
type WeatherPageData struct {
	templates.BaseData
	Cities      []City
	Selected    string
	Weather     *WeatherResponse
	Description string
	Updated     string
	Error       string
}

// NewWeatherPageData henter vejret for den valgte by (København som standard).
func NewWeatherPageData(ctx context.Context, cityName string) WeatherPageData {
	city := Cities[0]
	for _, c := range Cities {
		if c.Name == cityName {
			city = c
			break
		}
	}

	data := WeatherPageData{Cities: Cities, Selected: city.Name}

	w, err := fetchWeather(ctx, city.Lat, city.Lon)
	if err != nil {
		data.Error = "Kunne ikke hente vejret lige nu. Prøv igen senere."
		return data
	}
	data.Weather = w

	if d, ok := weatherDescriptions[w.WeatherCode]; ok {
		data.Description = d
	} else {
		data.Description = "Ukendt vejr"
	}

	if t, err := time.Parse("2006-01-02T15:04", w.Time); err == nil {
		data.Updated = t.Format("02-01-2006 15:04")
	} else {
		data.Updated = w.Time
	}
	return data
}

func WeatherPage(w http.ResponseWriter, r *http.Request) {
	data := NewWeatherPageData(r.Context(), r.URL.Query().Get("city"))

	if err := templates.Page("weather.html").ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}