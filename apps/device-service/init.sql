-- Схема Device DB для MVP.
-- id устройства совпадает с id датчика в монолите, пока устройства регистрируются из него.
CREATE TABLE IF NOT EXISTS devices (
    id         VARCHAR(64) PRIMARY KEY,
    house_id   UUID         NOT NULL,
    type       VARCHAR(50)  NOT NULL,
    name       VARCHAR(100) NOT NULL,
    location   VARCHAR(100) NOT NULL DEFAULT '',
    status     VARCHAR(20)  NOT NULL DEFAULT 'off',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(type);
