package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Entry struct {
	value     string
	expiresAt time.Time
}

type Store struct {
	mu   sync.RWMutex
	data map[string]Entry
	done chan struct{}
}

func newStore(sweepEvery time.Duration) *Store {
	s := &Store{data: make(map[string]Entry), done: make(chan struct{})}
	go s.SweepLoop(sweepEvery)
	return s
}

func (s *Store) SweepLoop(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.Sweep()
		case <-s.done:
			return
		}
	}
}

func (s *Store) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.data {
		if !v.expiresAt.IsZero() && time.Now().After(v.expiresAt) {
			delete(s.data, k)
		}
	}
}

func (s *Store) Set(key string, data string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = Entry{data, time.Now().Add(ttl)}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	if !ok {
		return "", false
	}
	if !val.expiresAt.IsZero() && time.Now().After(val.expiresAt) {
		return "", false
	}
	return val.value, ok

}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l := len(s.data)
	return l
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *Store) Stop() { close(s.done) }

type Server struct {
	store *Store
	start time.Time
}

func newServer(s *Store) *Server {
	return &Server{s, time.Now()}
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	val, ok := s.store.Get(key)
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	w.Write([]byte(val))
	log.Printf("got key: %s", key)
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	s.store.Set(key, string(body), time.Minute)

	w.WriteHeader(http.StatusCreated)
	log.Printf("put value: %s into key: %s\n", string(body), key)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	s.store.Delete(key)
	log.Printf("deleted key: %s\n", key)
}

type StatsResponse struct {
	KeyCount      int `json:"key_count"`
	UptimeSeconds int `json:"uptime_seconds"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	l := s.store.Len()
	upTime := int(time.Since(s.start).Seconds())
	resp := StatsResponse{l, upTime}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	log.Println("returned stats")

}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /store/{key}", s.handleGet)
	mux.HandleFunc("PUT /store/{key}", s.handlePut)
	mux.HandleFunc("DELETE /store/{key}", s.handleDelete)
	mux.HandleFunc("GET /stats", s.handleStats)
	return mux
}

func main() {
	store := newStore(50 * time.Millisecond)
	defer store.Stop()

	server := newServer(store)
	mux := server.routes()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Println("listening on :8080")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("stopped cleanly")

}
