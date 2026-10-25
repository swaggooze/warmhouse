import asyncpg

from app.models import Measurement


class TelemetryRepository:
    """Хранит измерения в Telemetry DB."""

    def __init__(self, dsn: str):
        self._dsn = dsn
        self._pool: asyncpg.Pool | None = None

    async def connect(self) -> None:
        self._pool = await asyncpg.create_pool(self._dsn, min_size=1, max_size=5)

    async def close(self) -> None:
        if self._pool:
            await self._pool.close()

    async def save(self, m: Measurement) -> None:
        # Повторно доставленное брокером измерение не создаёт дубль
        await self._pool.execute(
            """
            INSERT INTO measurements (house_id, device_id, metric, value, unit, measured_at)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (device_id, metric, measured_at) DO NOTHING
            """,
            m.houseId, m.deviceId, m.metric, m.value, m.unit, m.measuredAt,
        )

    async def latest(self, device_id: str, metric: str) -> Measurement | None:
        rows = await self.history(device_id, metric, limit=1)
        return rows[0] if rows else None

    async def history(self, device_id: str, metric: str, limit: int) -> list[Measurement]:
        rows = await self._pool.fetch(
            """
            SELECT house_id, device_id, metric, value, unit, measured_at
            FROM measurements
            WHERE device_id = $1 AND metric = $2
            ORDER BY measured_at DESC
            LIMIT $3
            """,
            device_id, metric, limit,
        )
        return [
            Measurement(
                houseId=str(r["house_id"]), deviceId=r["device_id"], metric=r["metric"],
                value=r["value"], unit=r["unit"], measuredAt=r["measured_at"],
            )
            for r in rows
        ]
