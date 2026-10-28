package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/doxanocap/blotato/internal/adapter"
	"github.com/doxanocap/blotato/internal/fake"
	"github.com/doxanocap/blotato/internal/scenario"
	"github.com/doxanocap/blotato/internal/sqlite"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	path := os.Getenv("BLOTATO_DB")
	if path == "" {
		path = "blotato.db"
	}
	store, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	defer store.Close()
	service := scenario.New(store.DB, scenario.Settings{Reply: "Check your DMs!", Question: "What is your email address?", Link: "https://example.com/pricing", Wait: 24 * time.Hour})
	sender := fake.New()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		var e adapter.Event
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&e); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if err := service.Receive(r.Context(), e); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, scenario.ErrInvalidEvent) {
				status = http.StatusBadRequest
			}
			http.Error(w, "event not accepted", status)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /fake/outcome", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Step    adapter.Step    `json:"step"`
			Outcome adapter.Outcome `json:"outcome"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in) != nil || (in.Step != adapter.ReplyComment && in.Step != adapter.AskEmail && in.Step != adapter.SendLink) || (in.Outcome != adapter.Accepted && in.Outcome != adapter.Retryable && in.Outcome != adapter.Rejected && in.Outcome != adapter.Unknown) {
			http.Error(w, "invalid outcome", 400)
			return
		}
		sender.Set(in.Step, in.Outcome)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /runs", func(w http.ResponseWriter, r *http.Request) {
		rows, err := store.DB.QueryContext(r.Context(), `SELECT r.id,r.state,r.error_code,COALESCE(a.step,''),COALESCE(a.status,''),COALESCE(a.error_code,'') FROM runs r LEFT JOIN actions a ON a.run_id=r.id ORDER BY r.id,a.id`)
		if err != nil {
			http.Error(w, "storage error", 500)
			return
		}
		defer rows.Close()
		type entry struct {
			RunID       int64  `json:"run_id"`
			State       string `json:"state"`
			Error       string `json:"error"`
			Step        string `json:"step"`
			Status      string `json:"status"`
			ActionError string `json:"action_error"`
		}
		out := []entry{}
		for rows.Next() {
			var e entry
			if err = rows.Scan(&e.RunID, &e.State, &e.Error, &e.Step, &e.Status, &e.ActionError); err != nil {
				http.Error(w, "storage error", 500)
				return
			}
			out = append(out, e)
		}
		if rows.Err() != nil {
			http.Error(w, "storage error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(out); err != nil {
			slog.Error("write response", "error", err)
		}
	})
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	workerErr := make(chan error, 1)
	serverErr := make(chan error, 1)
	go func() { workerErr <- service.Run(workerCtx, sender) }()
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErr <- err
	}()
	workerStopped := false
	select {
	case <-ctx.Done():
	case err = <-workerErr:
		workerStopped = true
	case err = <-serverErr:
	}
	if err != nil {
		err = fmt.Errorf("serve: %w", err)
	}
	stopWorker()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := server.Shutdown(shutdown); err == nil && e != nil {
		err = e
	}
	if !workerStopped {
		if e := <-workerErr; err == nil && e != nil && !errors.Is(e, context.Canceled) {
			err = e
		}
	}
	return err
}
