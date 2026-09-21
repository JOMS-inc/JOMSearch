package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const openMeteoURL = "https://api.open-meteo.com/v1/forecast"

// Default location: Copenhagen
const (
	defaultLat = 55.6761
	defaultLon = 12.5683
)

var weatherClient = &http.Client{Timeout: 5 * time.Second}

// What we get back from Open-Meteo
type openMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Current   struct {
		Time        string  `json:"time"`
		Temperature float64 `json:"temperature_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
}

// What our own API returns
type WeatherResponse struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Time        string  `json:"time"`
	Temperature float64 `json:"temperature_c"`
	WindSpeed   float64 `json:"wind_speed_kmh"`
	WeatherCode int     `json:"weather_code"`
}

// Weather handles GET /api/weather?lat=55.67&lon=12.56
func Weather(w http.ResponseWriter, r *http.Request) {
	lat, lon := defaultLat, defaultLon

	if v := r.URL.Query().Get("lat"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < -90 || f > 90 {
			http.Error(w, "invalid lat", http.StatusBadRequest)
			return
		}
		lat = f
	}
	if v := r.URL.Query().Get("lon"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < -180 || f > 180 {
			http.Error(w, "invalid lon", http.StatusBadRequest)
			return
		}
		lon = f
	}

	data, err := fetchWeather(r.Context(), lat, lon)
	if err != nil {
		http.Error(w, "weather service unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func fetchWeather(ctx context.Context, lat, lon float64) (*WeatherResponse, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,wind_speed_10m,weather_code")
	q.Set("timezone", "Europe/Copenhagen")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openMeteoURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := weatherClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call open-meteo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo returned status %d", resp.StatusCode)
	}

	var om openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&om); err != nil {
		return nil, fmt.Errorf("decode open-meteo response: %w", err)
	}

	return &WeatherResponse{
		Latitude:    om.Latitude,
		Longitude:   om.Longitude,
		Time:        om.Current.Time,
		Temperature: om.Current.Temperature,
		WindSpeed:   om.Current.WindSpeed,
		WeatherCode: om.Current.WeatherCode,
	}, nil
}