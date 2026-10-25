from datetime import datetime

from pydantic import BaseModel


class Measurement(BaseModel):
    """Измерение из канала device.measurements (см. schemas/api/device-events.asyncapi.yaml)."""

    houseId: str
    deviceId: str
    metric: str
    value: float
    unit: str
    measuredAt: datetime
