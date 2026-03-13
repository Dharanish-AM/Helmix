package eventsdk

import (
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestPublishAndSubscribe(t *testing.T) {
	natsURL, shutdown := startTestNATSServer(t)
	defer shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	received := make(chan RepoAnalyzedEvent, 1)
	err = Subscribe[RepoAnalyzedEvent](nc, string(RepoAnalyzed), func(e RepoAnalyzedEvent) {
		received <- e
		wg.Done()
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	event := RepoAnalyzedEvent{
		BaseEvent: BaseEvent{
			ID:        "evt-1",
			Type:      string(RepoAnalyzed),
			OrgID:     "org-1",
			ProjectID: "proj-1",
			CreatedAt: time.Now().UTC(),
		},
		RepoID: "repo-1",
		Stack: DetectedStack{
			Runtime:   "go",
			Framework: "gin",
		},
	}

	if err := Publish(nc, event); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if err := nc.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()

	select {
	case <-wait:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscription callback")
	}

	select {
	case got := <-received:
		if got.ID != event.ID {
			t.Fatalf("expected ID %q, got %q", event.ID, got.ID)
		}
		if got.RepoID != event.RepoID {
			t.Fatalf("expected RepoID %q, got %q", event.RepoID, got.RepoID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected event payload")
	}
}

func TestPublishFailsWithoutType(t *testing.T) {
	natsURL, shutdown := startTestNATSServer(t)
	defer shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()

	event := RepoAnalyzedEvent{
		BaseEvent: BaseEvent{ID: "evt-2"},
	}

	if err := Publish(nc, event); err == nil {
		t.Fatal("expected publish to fail when type is missing")
	}
}

func TestSubscribeInputValidation(t *testing.T) {
	natsURL, shutdown := startTestNATSServer(t)
	defer shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()

	if err := Subscribe[RepoAnalyzedEvent](nil, string(RepoAnalyzed), func(RepoAnalyzedEvent) {}); err == nil {
		t.Fatal("expected nil connection validation error")
	}

	if err := Subscribe[RepoAnalyzedEvent](nc, "", func(RepoAnalyzedEvent) {}); err == nil {
		t.Fatal("expected empty subject validation error")
	}

	if err := Subscribe[RepoAnalyzedEvent](nc, string(RepoAnalyzed), nil); err == nil {
		t.Fatal("expected nil handler validation error")
	}
}

func startTestNATSServer(t *testing.T) (string, func()) {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}

	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("new nats server: %v", err)
	}

	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server did not become ready")
	}

	url := srv.ClientURL()
	return url, func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	}
}
