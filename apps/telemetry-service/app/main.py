import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Query

from app.consumer import MeasurementConsumer
from app.models import Measurement
from app.repository import TelemetryRepository

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")

repository = TelemetryRepository(os.getenv("DATABASE_URL", "postgresql://telemetry:telemetry@localhost:5432/telemetry"))
consumer = MeasurementConsumer(os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), repository)


@asynccontextmanager
async def lifespan(app: FastAPI):
    await repository.connect()
    await consumer.start()
    yield
    await consumer.stop()
    await repository.close()


app = FastAPI(title="Telemetry Service", version="1.0.0", lifespan=lifespan)


@app.get("/health")
async def health() -> dict:
    return {"status": "ok"}


@app.get("/api/v1/devices/{device_id}/telemetry/latest", response_model=Measurement)
async def latest(device_id: str, metric: str = "temperature") -> Measurement:
    measurement = await repository.latest(device_id, metric)
    if measurement is None:
        raise HTTPException(status_code=404, detail="no measurements for device")
    return measurement


@app.get("/api/v1/devices/{device_id}/telemetry", response_model=list[Measurement])
async def history(device_id: str, metric: str = "temperature",
                  limit: int = Query(default=20, ge=1, le=1000)) -> list[Measurement]:
    return await repository.history(device_id, metric, limit)
