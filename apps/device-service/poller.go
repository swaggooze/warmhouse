package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Measurement - сообщение для канала device.measurements (см. schemas/api/device-events.asyncapi.yaml)
type Measurement struct {
	HouseID    string    `json:"houseId"`
	DeviceID   string    `json:"deviceId"`
	Metric     string    `json:"metric"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	MeasuredAt time.Time `json:"measuredAt"`
}

// LegacySensorPoller - адаптер для датчиков старого образца, которые сами данные не отправляют:
// периодически опрашивает их по HTTP и публикует измерения в брокер.
// В целевой архитектуре эту роль выполняет Device Gateway.
type LegacySensorPoller struct {
	repo       *DeviceRepository
	publisher  Publisher
	sensorsURL string
	interval   time.Duration
	client     *http.Client
}

func NewLegacySensorPoller(repo *DeviceRepository, publisher Publisher, sensorsURL string, interval time.Duration) *LegacySensorPoller {
	return &LegacySensorPoller{
		repo:       repo,
		publisher:  publisher,
		sensorsURL: sensorsURL,
		interval:   interval,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *LegacySensorPoller) Run(ctx context.Context) {
	log.Printf("Polling temperature sensors at %s every %s", p.sensorsURL, p.interval)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *LegacySensorPoller) pollOnce(ctx context.Context) {
	devices, err := p.repo.List(ctx, "temperature")
	if err != nil {
		log.Printf("Failed to list temperature devices: %v", err)
		return
	}
	for _, d := range devices {
		m, err := p.read(ctx, d)
		if err != nil {
			log.Printf("Failed to read sensor %s: %v", d.ID, err)
			continue
		}
		routingKey := fmt.Sprintf("%s.%s.%s", d.HouseID, d.ID, m.Metric)
		if err := p.publisher.Publish(ctx, MeasurementsExchange, routingKey, m); err != nil {
			log.Printf("Failed to publish measurement of sensor %s: %v", d.ID, err)
		}
	}
}

// read запрашивает показание датчика в формате temperature-api
func (p *LegacySensorPoller) read(ctx context.Context, d Device) (Measurement, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/temperature/%s", p.sensorsURL, d.ID), nil)
	if err != nil {
		return Measurement{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return Measurement{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Measurement{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var reading struct {
		Value     float64   `json:"value"`
		Unit      string    `json:"unit"`
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reading); err != nil {
		return Measurement{}, err
	}
	return Measurement{
		HouseID:    d.HouseID,
		DeviceID:   d.ID,
		Metric:     "temperature",
		Value:      reading.Value,
		Unit:       reading.Unit,
		MeasuredAt: reading.Timestamp,
	}, nil
}
