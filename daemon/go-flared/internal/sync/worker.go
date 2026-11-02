package sync

import (
	"time"

	"flare/go-flared/internal/db"
)

func Worker() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		_, _ = db.DB.Exec(`UPDATE offline_sync_queue SET status=1, attempts=attempts+1, last_attempt_at=CURRENT_TIMESTAMP WHERE status=0 AND queue_id IN (SELECT queue_id FROM offline_sync_queue WHERE status=0 ORDER BY created_at LIMIT 5)`)
		_, _ = db.DB.Exec(`UPDATE offline_sync_queue SET status=2 WHERE status=1`)
	}
}
