-- Схема Telemetry DB для MVP: измерения хранятся во временном ряду TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS measurements (
    house_id    UUID             NOT NULL,
    device_id   VARCHAR(64)      NOT NULL,
    metric      VARCHAR(50)      NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    unit        VARCHAR(20)      NOT NULL,
    measured_at TIMESTAMPTZ      NOT NULL,
    PRIMARY KEY (device_id, metric, measured_at)
);

SELECT create_hypertable('measurements', 'measured_at', if_not_exists => TRUE);
