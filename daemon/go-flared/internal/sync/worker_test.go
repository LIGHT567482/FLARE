package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"flare/go-flared/internal/db"
)

func TestValidateEndpointRequiresSecureRemoteTransport(t *testing.T) {
	tests := []struct {
		endpoint string
		wantErr  bool
	}{
		{"https://sync.example.invalid", false},
		{"http://127.0.0.1:8000", false},
		{"http://sync.example.invalid", true},
		{"https://user:password@sync.example.invalid", true},
	}
	for _, test := range tests {
		err := validateEndpoint(test.endpoint)
		if (err != nil) != test.wantErr {
			t.Errorf("validateEndpoint(%q) error = %v, wantErr %v", test.endpoint, err, test.wantErr)
		}
	}
}

func TestProcessBatchMarksEventsDeliveredAfterAcknowledgement(t *testing.T) {
	setUpQueueDB(t)
	_, err := db.DB.Exec(`INSERT INTO offline_sync_queue
(queue_id, entity_type, payload_json, status, attempts, created_at)
VALUES ('evt-1', 'note', '{"entity_id":"n1"}', 0, 0, CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected authorization header")
		}
		var request pushRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode sync request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(request.Items) != 1 || request.Items[0].QueueID != "evt-1" {
			t.Errorf("unexpected sync batch: %#v", request.Items)
		}
		_ = json.NewEncoder(w).Encode(pushResponse{Accepted: []string{"evt-1"}})
	}))
	defer server.Close()

	if err := processBatch(server.Client(), server.URL, "test-token"); err != nil {
		t.Fatal(err)
	}
	var status, attempts int
	if err := db.DB.QueryRow(`SELECT status, attempts FROM offline_sync_queue WHERE queue_id='evt-1'`).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != 2 || attempts != 1 {
		t.Fatalf("got status=%d attempts=%d, want delivered after one attempt", status, attempts)
	}
}

func TestProcessBatchLeavesFailedEventsRetryable(t *testing.T) {
	setUpQueueDB(t)
	_, err := db.DB.Exec(`INSERT INTO offline_sync_queue
(queue_id, entity_type, payload_json, status, attempts, created_at)
VALUES ('evt-2', 'note', '{"entity_id":"n2"}', 0, 0, CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if err := processBatch(server.Client(), server.URL, "test-token"); err == nil {
		t.Fatal("expected remote error")
	}
	var status, attempts int
	if err := db.DB.QueryRow(`SELECT status, attempts FROM offline_sync_queue WHERE queue_id='evt-2'`).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != 3 || attempts != 1 {
		t.Fatalf("got status=%d attempts=%d, want retryable failure after one attempt", status, attempts)
	}
}

func TestConflictListAndResolutionUseAuthenticatedRemoteAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/sync/conflicts/":
			_, _ = fmt.Fprint(w, `{"conflicts":[{"conflict_id":"conflict-1","entity_type":"note","entity_id":"n1","event_ids":["evt-a","evt-b"],"payloads":[{"text":"a"},{"text":"b"}],"created_at":"2026-11-02T12:00:00Z","resolved":false}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/sync/conflicts/conflict-1/resolve/":
			var request resolveRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if request.EventID != "evt-b" {
				http.Error(w, "wrong event", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"resolved":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("FLARE_SYNC_URL", server.URL)
	t.Setenv("FLARE_SYNC_TOKEN", "test-token")

	conflicts, err := ListConflicts()
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || conflicts[0].ConflictID != "conflict-1" {
		t.Fatalf("unexpected conflict list: %#v", conflicts)
	}
	if err := ResolveConflict("conflict-1", "evt-b"); err != nil {
		t.Fatal(err)
	}
}

func setUpQueueDB(t *testing.T) {
	t.Helper()
	previous := db.DB
	if err := db.Init(filepath.Join(t.TempDir(), "sync-test.db")); err != nil {
		t.Fatal(err)
	}
	current := db.DB
	t.Cleanup(func() {
		_ = current.Close()
		db.DB = previous
	})
}
