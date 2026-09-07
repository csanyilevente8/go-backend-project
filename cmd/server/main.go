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
	"syscall"
	"time"

	"github.com/csanyilevente8/go-backend-project/internal/db"
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
	handler := httpapi.NewTodoHandler(repo)
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
