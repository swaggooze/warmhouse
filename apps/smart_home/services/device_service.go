package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"smarthome/models"
)

// DeviceService forwards sensor changes to the device-service microservice
// while device management is being moved out of the monolith
type DeviceService struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewDeviceService creates a new device service client; empty baseURL disables it
func NewDeviceService(baseURL string) *DeviceService {
	return &DeviceService{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// Enabled reports whether the device-service URL is configured
func (s *DeviceService) Enabled() bool {
	return s.BaseURL != ""
}

// RegisterSensor registers a sensor as a device in device-service
func (s *DeviceService) RegisterSensor(sensor models.Sensor) error {
	return s.post("/api/v1/devices", map[string]string{
		"id":       strconv.Itoa(sensor.ID),
		"type":     string(sensor.Type),
		"name":     sensor.Name,
		"location": sensor.Location,
	}, http.StatusCreated)
}

// SwitchSensor sends an on/off command for a sensor's device to device-service
func (s *DeviceService) SwitchSensor(sensorID int, on bool) error {
	value := "off"
	if on {
		value = "on"
	}
	return s.post(fmt.Sprintf("/api/v1/devices/%d/commands", sensorID), map[string]string{
		"capability": "switch",
		"value":      value,
	}, http.StatusAccepted)
}

func (s *DeviceService) post(path string, payload any, expectedStatus int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.HTTPClient.Post(s.BaseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error calling device-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("unexpected status code from device-service: %d", resp.StatusCode)
	}
	return nil
}
