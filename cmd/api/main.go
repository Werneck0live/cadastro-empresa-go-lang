package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Werneck0live/cadastro-empresa/internal/broker"
	"github.com/Werneck0live/cadastro-empresa/internal/config"
	"github.com/Werneck0live/cadastro-empresa/internal/db"
	"github.com/Werneck0live/cadastro-empresa/internal/handlers"
	"github.com/Werneck0live/cadastro-empresa/internal/repository"
)

func main() {
	cfg := config.Load()

	client, err := db.NewMongoClient(cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect error: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	database := client.Database(cfg.MongoDB)
	// ensureIndexes(database)

	// RabbitMQ "publicar a mensagem" - instanciando a função
	pub, err := broker.NewPublisher(cfg.RabbitURI, cfg.RabbitQueue)
	if err != nil {
		log.Fatalf("rabbitmq connect error: %v", err)
	}

	defer func() { _ = pub.Close() }()

	repo := repository.NewCompanyRepository(database)
	// h := &handlers.CompanyHandler{Repo: repo, Pub: pub}
	h := &handlers.CompanyHandler{repo, pub}

	// Padronização das requisições
	mux := http.NewServeMux()
	// mux.HandleFunc("/healthz", h.Health)
	mux.HandleFunc("/api/companies", h.Companies)
	mux.HandleFunc("/api/companies/", h.CompanyByID)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		log.Printf("%s %s %s", r.Method, r.URL.Path, fmtDuration(dur))
	})
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}
