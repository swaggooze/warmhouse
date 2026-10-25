import asyncio
import logging

import aio_pika
from pydantic import ValidationError

from app.models import Measurement
from app.repository import TelemetryRepository

log = logging.getLogger(__name__)

EXCHANGE = "device.measurements"
QUEUE = "telemetry.measurements"


class MeasurementConsumer:
    """Получает измерения из RabbitMQ и сохраняет их в репозиторий."""

    def __init__(self, url: str, repository: TelemetryRepository):
        self._url = url
        self._repository = repository
        self._connection: aio_pika.abc.AbstractRobustConnection | None = None

    async def start(self) -> None:
        self._connection = await self._connect()
        channel = await self._connection.channel()
        await channel.set_qos(prefetch_count=50)

        exchange = await channel.declare_exchange(EXCHANGE, aio_pika.ExchangeType.TOPIC, durable=True)
        queue = await channel.declare_queue(QUEUE, durable=True)
        await queue.bind(exchange, routing_key="#")
        await queue.consume(self._on_message)
        log.info("Consuming measurements from exchange %s", EXCHANGE)

    async def stop(self) -> None:
        if self._connection:
            await self._connection.close()

    async def _connect(self) -> aio_pika.abc.AbstractRobustConnection:
        for attempt in range(1, 11):
            try:
                return await aio_pika.connect_robust(self._url)
            except (OSError, aio_pika.exceptions.AMQPConnectionError) as e:
                log.warning("RabbitMQ is not ready (attempt %d): %s", attempt, e)
                await asyncio.sleep(2)
        raise RuntimeError("Unable to connect to RabbitMQ")

    async def _on_message(self, message: aio_pika.abc.AbstractIncomingMessage) -> None:
        # Сообщение подтверждается после записи в БД; при ошибке БД брокер доставит его снова
        async with message.process(requeue=True):
            try:
                measurement = Measurement.model_validate_json(message.body)
            except ValidationError as e:
                log.error("Skipping invalid measurement: %s", e)
                return
            await self._repository.save(measurement)
            log.info("Stored %s=%s %s for device %s",
                     measurement.metric, measurement.value, measurement.unit, measurement.deviceId)
