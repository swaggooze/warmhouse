package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	CommandsExchange     = "device.commands"
	MeasurementsExchange = "device.measurements"
)

// Publisher отправляет сообщения в брокер; интерфейс позволяет подменить брокер в тестах
type Publisher interface {
	Publish(ctx context.Context, exchange, routingKey string, payload any) error
}

// RabbitPublisher публикует JSON-сообщения в topic exchange RabbitMQ
type RabbitPublisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewRabbitPublisher подключается к брокеру с повторами и объявляет exchange
func NewRabbitPublisher(url string) (*RabbitPublisher, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		if conn, err = amqp.Dial(url); err == nil {
			break
		}
		log.Printf("RabbitMQ is not ready (attempt %d): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	for _, name := range []string{CommandsExchange, MeasurementsExchange} {
		if err := ch.ExchangeDeclare(name, "topic", true, false, false, false, nil); err != nil {
			conn.Close()
			return nil, fmt.Errorf("declare exchange %s: %w", name, err)
		}
	}
	return &RabbitPublisher{conn: conn, ch: ch}, nil
}

func (p *RabbitPublisher) Publish(ctx context.Context, exchange, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	})
}

func (p *RabbitPublisher) Close() {
	p.ch.Close()
	p.conn.Close()
}
