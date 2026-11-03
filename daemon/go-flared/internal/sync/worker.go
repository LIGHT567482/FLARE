package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"flare/go-flared/internal/db"
)

const (
	batchSize     = 25
	maxAttempts   = 8
	maxBatchBytes = 900 * 1024
)

type syncItem struct {
	QueueID    string          `json:"queue_id"`
	EntityType string          `json:"entity_type"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  string          `json:"created_at"`
}

type pushRequest struct {
	Items []syncItem `json:"items"`
}

type pushResponse struct {
	Accepted []string `json:"accepted"`
}

type Conflict struct {
	ConflictID string            `json:"conflict_id"`
	EntityType string            `json:"entity_type"`
	EntityID   string            `json:"entity_id"`
	EventIDs   []string          `json:"event_ids"`
	Payloads   []json.RawMessage `json:"payloads"`
	CreatedAt  string            `json:"created_at"`
	Resolved   bool              `json:"resolved"`
}

type conflictListResponse struct {
	Conflicts []Conflict `json:"conflicts"`
}

type resolveRequest struct {
	EventID string `json:"event_id"`
}

func Worker() {
	baseURL, token, client, err := remoteConfig()
	if err != nil {
		if os.Getenv("FLARE_SYNC_URL") == "" && os.Getenv("FLARE_SYNC_TOKEN") == "" {
			log.Printf("[sync] remote sync is not configured; queued events remain local")
			return
		}
		log.Printf("[sync] remote sync configuration error: %v", err)
		return
	}
	endpoint := baseURL + "/api/sync/push/"
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := processBatch(client, endpoint, token); err != nil {
			log.Printf("[sync] batch delivery failed: %v", err)
		}
		<-ticker.C
	}
}

func remoteConfig() (string, string, *http.Client, error) {
	endpoint, token := os.Getenv("FLARE_SYNC_URL"), os.Getenv("FLARE_SYNC_TOKEN")
	if endpoint == "" || token == "" {
		if endpoint == "" && token == "" {
			return "", "", nil, errors.New("sync is not configured")
		}
		return "", "", nil, errors.New("both FLARE_SYNC_URL and FLARE_SYNC_TOKEN are required")
	}
	if err := validateEndpoint(endpoint); err != nil {
		return "", "", nil, err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return strings.TrimRight(endpoint, "/"), token, client, nil
}

func remoteRequest(client *http.Client, method, endpoint, token string, body io.Reader, result any) error {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return fmt.Errorf("create sync request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send sync request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read sync response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sync server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if result != nil {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("decode sync response: %w", err)
		}
	}
	return nil
}

func ListConflicts() ([]Conflict, error) {
	baseURL, token, client, err := remoteConfig()
	if err != nil {
		return nil, err
	}
	var response conflictListResponse
	if err := remoteRequest(client, http.MethodGet, baseURL+"/api/sync/conflicts/", token, nil, &response); err != nil {
		return nil, err
	}
	return response.Conflicts, nil
}

func ResolveConflict(conflictID, eventID string) error {
	baseURL, token, client, err := remoteConfig()
	if err != nil {
		return err
	}
	if conflictID == "" || eventID == "" {
		return errors.New("conflict_id and event_id are required")
	}
	body, err := json.Marshal(resolveRequest{EventID: eventID})
	if err != nil {
		return fmt.Errorf("encode conflict resolution: %w", err)
	}
	var result struct {
		Resolved bool `json:"resolved"`
	}
	endpoint := baseURL + "/api/sync/conflicts/" + url.PathEscape(conflictID) + "/resolve/"
	if err := remoteRequest(client, http.MethodPost, endpoint, token, bytes.NewReader(body), &result); err != nil {
		return err
	}
	if !result.Resolved {
		return errors.New("sync server did not confirm conflict resolution")
	}
	return nil
}

func validateEndpoint(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("FLARE_SYNC_URL must be an origin or base path without credentials, query, or fragment")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" {
		host, _, splitErr := net.SplitHostPort(parsed.Host)
		if splitErr != nil {
			host = parsed.Host
		}
		if host == "localhost" || net.ParseIP(host).IsLoopback() {
			return nil
		}
	}
	return errors.New("remote sync requires HTTPS; HTTP is allowed only on loopback")
}

func processBatch(client *http.Client, endpoint, token string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin queue transaction: %w", err)
	}
	rows, err := tx.Query(`SELECT queue_id, entity_type, payload_json, created_at
FROM offline_sync_queue
WHERE status = 0 OR (
  status = 3 AND attempts < ? AND
  CAST(strftime('%s','now') AS INTEGER) -
  COALESCE(CAST(strftime('%s', last_attempt_at) AS INTEGER), 0) >= MIN(3600, 5 * (1 << attempts))
)
ORDER BY created_at
LIMIT ?`, maxAttempts, batchSize)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("select pending sync events: %w", err)
	}
	items := make([]syncItem, 0, batchSize)
	for rows.Next() {
		var item syncItem
		var payload string
		if err := rows.Scan(&item.QueueID, &item.EntityType, &payload, &item.CreatedAt); err != nil {
			_ = rows.Close()
			_ = tx.Rollback()
			return fmt.Errorf("read sync event: %w", err)
		}
		item.Payload = json.RawMessage(payload)
		if !json.Valid(item.Payload) {
			_ = rows.Close()
			_ = tx.Rollback()
			return fmt.Errorf("queue event %s contains invalid JSON", item.QueueID)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		_ = tx.Rollback()
		return fmt.Errorf("iterate sync events: %w", err)
	}
	if err := rows.Close(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("close sync query: %w", err)
	}
	if len(items) == 0 {
		return tx.Commit()
	}
	batch := make([]syncItem, 0, len(items))
	for _, item := range items {
		candidate := append(append([]syncItem(nil), batch...), item)
		encoded, err := json.Marshal(pushRequest{Items: candidate})
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("encode sync batch: %w", err)
		}
		if len(encoded) > maxBatchBytes {
			if len(batch) == 0 {
				if _, err := tx.Exec(`UPDATE offline_sync_queue
SET status = 3, attempts = ?, last_attempt_at = CURRENT_TIMESTAMP
WHERE queue_id = ? AND status IN (0, 3)`, maxAttempts, item.QueueID); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("quarantine oversized event %s: %w", item.QueueID, err)
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("commit oversized event failure: %w", err)
				}
				return fmt.Errorf("queue event %s exceeds the %d-byte sync batch limit", item.QueueID, maxBatchBytes)
			}
			break
		}
		batch = append(batch, item)
	}
	items = batch
	for _, item := range items {
		if _, err := tx.Exec(`UPDATE offline_sync_queue
SET status = 1, attempts = attempts + 1, last_attempt_at = CURRENT_TIMESTAMP
WHERE queue_id = ? AND status IN (0, 3)`, item.QueueID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("claim sync event %s: %w", item.QueueID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit queue claim: %w", err)
	}

	body, err := json.Marshal(pushRequest{Items: items})
	if err != nil {
		return failBatch(items, fmt.Errorf("encode sync batch: %w", err))
	}
	var result pushResponse
	if err := remoteRequest(client, http.MethodPost, endpoint, token, bytes.NewReader(body), &result); err != nil {
		return failBatch(items, err)
	}
	accepted := make(map[string]bool, len(result.Accepted))
	for _, id := range result.Accepted {
		accepted[id] = true
	}
	for _, item := range items {
		if !accepted[item.QueueID] {
			return failBatch(items, fmt.Errorf("server did not acknowledge event %s", item.QueueID))
		}
	}

	for _, item := range items {
		result, err := db.DB.Exec(`UPDATE offline_sync_queue SET status = 2 WHERE queue_id = ? AND status = 1`, item.QueueID)
		if err != nil {
			return fmt.Errorf("mark event %s delivered: %w", item.QueueID, err)
		}
		if changed, err := result.RowsAffected(); err != nil || changed != 1 {
			return fmt.Errorf("event %s was not in processing state after remote acknowledgement", item.QueueID)
		}
	}
	return nil
}

func failBatch(items []syncItem, cause error) error {
	for _, item := range items {
		if _, err := db.DB.Exec(`UPDATE offline_sync_queue
SET status = 3, last_attempt_at = CURRENT_TIMESTAMP
WHERE queue_id = ? AND status = 1`, item.QueueID); err != nil {
			return fmt.Errorf("%v; mark event %s failed: %w", cause, item.QueueID, err)
		}
	}
	return cause
}
