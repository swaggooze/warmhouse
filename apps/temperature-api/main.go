package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// TemperatureResponse mirrors services.TemperatureResponse in the smart_home monolith
type TemperatureResponse struct {
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Timestamp   time.Time `json:"timestamp"`
	Location    string    `json:"location"`
	Status      string    `json:"status"`
	SensorID    string    `json:"sensor_id"`
	SensorType  string    `json:"sensor_type"`
	Description string    `json:"description"`
}

// Location <-> sensor ID mapping from the task description
var locationBySensor = map[string]string{
	"1": "Living Room",
	"2": "Bedroom",
	"3": "Kitchen",
}

var sensorByLocation = map[string]string{
	"Living Room": "1",
	"Bedroom":     "2",
	"Kitchen":     "3",
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// GET /temperature?location=Kitchen&sensorId=3 — both parameters are optional
	mux.HandleFunc("GET /temperature", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		writeJSON(w, http.StatusOK, reading(q.Get("location"), q.Get("sensorId")))
	})

	// GET /temperature/{sensorId} — used by the monolith for Get Sensor / Get All Sensors
	mux.HandleFunc("GET /temperature/{sensorId}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, reading("", r.PathValue("sensorId")))
	})

	srv := &http.Server{
		Addr:    ":" + getEnv("PORT", "8081"),
		Handler: logRequests(mux),
	}

	go func() {
		log.Printf("Temperature API starting on %s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v\n", err)
	}
}

// reading fills in the missing location or sensor ID and returns a random temperature
func reading(location, sensorID string) TemperatureResponse {
	if location == "" {
		location = locationBySensor[sensorID]
		if location == "" {
			location = "Unknown"
		}
	}
	if sensorID == "" {
		sensorID = sensorByLocation[location]
		if sensorID == "" {
			sensorID = "0"
		}
	}

	value := float64(int((10+rand.Float64()*20)*10)) / 10

	return TemperatureResponse{
		Value:       value,
		Unit:        "°C",
		Timestamp:   time.Now().UTC(),
		Location:    location,
		Status:      "active",
		SensorID:    sensorID,
		SensorType:  "temperature",
		Description: "Simulated temperature reading for " + location,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start))
	})
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
