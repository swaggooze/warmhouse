package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// DeviceHandler - REST API сервиса устройств
type DeviceHandler struct {
	repo      *DeviceRepository
	publisher Publisher
}

func NewDeviceHandler(repo *DeviceRepository, publisher Publisher) *DeviceHandler {
	return &DeviceHandler{repo: repo, publisher: publisher}
}

func (h *DeviceHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/devices", h.createDevice)
	mux.HandleFunc("GET /api/v1/devices", h.listDevices)
	mux.HandleFunc("GET /api/v1/devices/{id}", h.getDevice)
	mux.HandleFunc("POST /api/v1/devices/{id}/commands", h.sendCommand)
}

type createDeviceRequest struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

func (h *DeviceHandler) createDevice(w http.ResponseWriter, r *http.Request) {
	var req createDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" || req.Type == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "id, type and name are required")
		return
	}

	device, err := h.repo.Save(r.Context(), Device{
		ID: req.ID, HouseID: LegacyHouseID, Type: req.Type, Name: req.Name, Location: req.Location,
	})
	if err != nil {
		log.Printf("Failed to save device %s: %v", req.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to save device")
		return
	}
	log.Printf("Device %s registered (%s, %s)", device.ID, device.Type, device.Location)
	writeJSON(w, http.StatusCreated, device)
}

func (h *DeviceHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.repo.List(r.Context(), r.URL.Query().Get("type"))
	if err != nil {
		log.Printf("Failed to list devices: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) getDevice(w http.ResponseWriter, r *http.Request) {
	device, err := h.repo.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		log.Printf("Failed to get device: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}
	writeJSON(w, http.StatusOK, device)
}

type commandRequest struct {
	Capability string `json:"capability"`
	Value      string `json:"value"`
}

// DeviceCommand - сообщение для канала device.commands (см. schemas/api/device-events.asyncapi.yaml)
type DeviceCommand struct {
	CommandID  string    `json:"commandId"`
	HouseID    string    `json:"houseId"`
	DeviceID   string    `json:"deviceId"`
	Capability string    `json:"capability"`
	Value      string    `json:"value"`
	IssuedAt   time.Time `json:"issuedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// sendCommand принимает команду "switch on/off": сохраняет желаемое состояние и публикует команду в брокер
func (h *DeviceHandler) sendCommand(w http.ResponseWriter, r *http.Request) {
	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Capability != "switch" || (req.Value != "on" && req.Value != "off") {
		writeError(w, http.StatusUnprocessableEntity, `only capability "switch" with value "on" or "off" is supported`)
		return
	}

	device, err := h.repo.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		log.Printf("Failed to get device: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}

	if err := h.repo.SetStatus(r.Context(), device.ID, req.Value); err != nil {
		log.Printf("Failed to save desired state of device %s: %v", device.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to save device state")
		return
	}

	now := time.Now().UTC()
	cmd := DeviceCommand{
		CommandID:  uuid.NewString(),
		HouseID:    device.HouseID,
		DeviceID:   device.ID,
		Capability: req.Capability,
		Value:      req.Value,
		IssuedAt:   now,
		ExpiresAt:  now.Add(30 * time.Second),
	}
	routingKey := fmt.Sprintf("%s.%s.command", device.HouseID, device.ID)
	if err := h.publisher.Publish(r.Context(), CommandsExchange, routingKey, cmd); err != nil {
		log.Printf("Failed to publish command for device %s: %v", device.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to send command")
		return
	}
	log.Printf("Command %s: %s=%s published for device %s", cmd.CommandID, cmd.Capability, cmd.Value, device.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{"commandId": cmd.CommandID, "status": "pending"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
