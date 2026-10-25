package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := sql.Open("pgx", getEnv("DATABASE_URL", "postgres://devices:devices@localhost:5432/devices"))
	if err != nil {
		log.Fatalf("Unable to open database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	log.Println("Connected to database successfully")

	publisher, err := NewRabbitPublisher(getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		log.Fatalf("Unable to connect to RabbitMQ: %v", err)
	}
	defer publisher.Close()
	log.Println("Connected to RabbitMQ successfully")

	repo := NewDeviceRepository(db)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	interval, err := time.ParseDuration(getEnv("POLL_INTERVAL", "10s"))
	if err != nil {
		log.Fatalf("Invalid POLL_INTERVAL: %v", err)
	}
	poller := NewLegacySensorPoller(repo, publisher, getEnv("TEMPERATURE_API_URL", "http://localhost:8081"), interval)
	go poller.Run(ctx)

	mux := http.NewServeMux()
	NewDeviceHandler(repo, publisher).Register(mux)

	srv := &http.Server{
		Addr:    ":" + getEnv("PORT", "8082"),
		Handler: mux,
	}

	go func() {
		log.Printf("Device service starting on %s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v\n", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
