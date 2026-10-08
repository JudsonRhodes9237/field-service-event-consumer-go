package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/field-service-event-consumer-go/internal/fieldservice"
	"github.com/example/field-service-event-consumer-go/internal/infrai"
)

func main() {
	var (
		addr            = flag.String("addr", ":8080", "listen address")
		baseURL         = flag.String("base-url", "", "public base URL for webhook registration")
		queueName       = flag.String("queue", "fieldservice-events", "queue name")
		webhookSecret   = flag.String("webhook-secret", "", "shared webhook secret")
		consumeInterval = flag.Duration("consume-interval", 2*time.Second, "queue consume interval")
	)
	flag.Parse()

	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	if *baseURL == "" {
		log.Fatal("-base-url is required")
	}
	if *webhookSecret == "" {
		log.Fatal("-webhook-secret is required")
	}

	client := infrai.NewClient(apiKey)
	ctx := context.Background()

	if _, err := client.RegisterWebhook(ctx, infrai.RegisterWebhookRequest{
		URL:    *baseURL + "/webhooks/field-events",
		Events: []string{"fieldservice.workorder.updated"},
		Secret: *webhookSecret,
	}); err != nil {
		log.Fatal(err)
	}

	if err := client.QueuePushSubscribe(ctx, *queueName, *baseURL+"/webhooks/field-events"); err != nil {
		log.Fatal(err)
	}

	store := fieldservice.NewWorkOrderStore()
	processor := fieldservice.NewProcessor(store)
	consumer := fieldservice.NewQueueWorker(client, *queueName, processor)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhooks/field-events", func(w http.ResponseWriter, r *http.Request) {
		status, err := fieldservice.HandleSignedEvent(r, *webhookSecret, client, *queueName)
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, fieldservice.ErrUnauthorized) {
				code = http.StatusUnauthorized
			}
			http.Error(w, err.Error(), code)
			return
		}
		w.WriteHeader(status)
	})
	mux.HandleFunc("GET /work-orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		state, ok := store.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	})

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	go func() {
		ticker := time.NewTicker(*consumeInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := consumer.RunOnce(context.Background()); err != nil {
				log.Printf("queue consume: %v", err)
			}
		}
	}()

	go func() {
		log.Printf("listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
