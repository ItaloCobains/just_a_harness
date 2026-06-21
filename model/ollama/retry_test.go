package ollama

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"harness/agent"
)

func resilientModel(endpoint string) Model {
	m := New("test", endpoint)
	m.BackoffBase = time.Millisecond
	return m
}

func TestNextRetriesTransient5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(chatBody("hello")))
	}))
	defer srv.Close()

	step, err := resilientModel(srv.URL).Next(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if step.Text != "hello" {
		t.Fatalf("text = %q, want hello", step.Text)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
}

func TestNextExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := resilientModel(srv.URL).Next(context.Background(), nil, nil, nil)
	if !errors.Is(err, agent.ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
}

func TestNextNoRetryOn4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := resilientModel(srv.URL).Next(context.Background(), nil, nil, nil); err == nil {
		t.Fatal("expected error on 4xx")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestNextCanceledContextNoRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resilientModel(srv.URL).Next(ctx, nil, nil, nil); err == nil {
		t.Fatal("expected error on canceled context")
	}
}

func TestNextRetriesStalledStreamBeforeFirstToken(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush() // send headers, then go silent
			time.Sleep(200 * time.Millisecond)
			return
		}
		w.Write([]byte(chatBody("hi")))
	}))
	defer srv.Close()

	m := resilientModel(srv.URL)
	m.IdleTimeout = 40 * time.Millisecond

	step, err := m.Next(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if step.Text != "hi" {
		t.Fatalf("text = %q, want hi", step.Text)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2 (one stall + one success)", got)
	}
}
