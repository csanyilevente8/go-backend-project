package main

// @title			Todo API (Go)
// @version		1.0
// @description	Go port of the Spring Boot Todo backend. Same REST contract.
// @BasePath		/
//
// To (re)generate the OpenAPI spec + Swagger UI assets:
//
//	go install github.com/swaggo/swag/cmd/swag@latest
//	swag init -g cmd/server/main.go -o internal/docs
//
// Then enable the UI route in internal/httpapi/router.go (see README).

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/csanyilevente8/go-backend-project/internal/db"
	"github.com/csanyilevente8/go-backend-project/internal/events"
	"github.com/csanyilevente8/go-backend-project/internal/httpapi"
	"github.com/csanyilevente8/go-backend-project/internal/repository"
)

func main() {
	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()
	log.Println("connected to database")

	if err := db.RunMigrations(pool); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}
	log.Println("migrations applied")

	repo := repository.NewTodoRepository(pool)
	activityRepo := repository.NewActivityRepository(pool)

	// Kafka is optional: if KAFKA_BROKERS is set, publish domain events and run
	// the activity consumer; otherwise use a no-op publisher so the app runs
	// unchanged without Kafka.
	var publisher events.Publisher = events.NoopPublisher{}
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if brokersEnv != "" {
		brokers := strings.Split(brokersEnv, ",")
		publisher = events.NewKafkaPublisher(brokers)
		log.Printf("kafka enabled, brokers=%s", brokersEnv)

		consumer := events.NewConsumer(brokers, activityRepo)
		go consumer.Run(ctx)
		defer consumer.Close()
	} else {
		log.Println("KAFKA_BROKERS not set; events disabled")
	}
	defer publisher.Close()

	handler := httpapi.NewTodoHandler(repo, activityRepo, publisher)
	router := httpapi.NewRouter(handler)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown on SIGTERM/SIGINT (matters for K8s rolling updates).
	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
