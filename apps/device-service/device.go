package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LegacyHouseID - дом по умолчанию для датчиков, перенесённых из монолита, где домов ещё нет
const LegacyHouseID = "00000000-0000-0000-0000-000000000001"

var ErrNotFound = errors.New("device not found")

// Device - устройство в доме
type Device struct {
	ID        string    `json:"id"`
	HouseID   string    `json:"houseId"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DeviceRepository хранит устройства в Device DB
type DeviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

const deviceColumns = `id, house_id, type, name, location, status, created_at, updated_at`

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var d Device
	err := row.Scan(&d.ID, &d.HouseID, &d.Type, &d.Name, &d.Location, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

// Save создаёт устройство или обновляет его описание, если оно уже зарегистрировано
func (r *DeviceRepository) Save(ctx context.Context, d Device) (Device, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO devices (id, house_id, type, name, location)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE
		SET type = EXCLUDED.type, name = EXCLUDED.name, location = EXCLUDED.location, updated_at = now()
		RETURNING `+deviceColumns,
		d.ID, d.HouseID, d.Type, d.Name, d.Location)
	return scanDevice(row)
}

func (r *DeviceRepository) Get(ctx context.Context, id string) (Device, error) {
	d, err := scanDevice(r.db.QueryRowContext(ctx, `SELECT `+deviceColumns+` FROM devices WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return d, err
}

func (r *DeviceRepository) List(ctx context.Context, deviceType string) ([]Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices`
	args := []any{}
	if deviceType != "" {
		query += ` WHERE type = $1`
		args = append(args, deviceType)
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// SetStatus сохраняет желаемое состояние устройства
func (r *DeviceRepository) SetStatus(ctx context.Context, id, status string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE devices SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
