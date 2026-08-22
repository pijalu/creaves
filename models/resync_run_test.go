package models

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
)

func TestResyncRunStringFieldsAndComplete(t *testing.T) {
	run := ResyncRun{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "center-test",
		Status:     "running",
		StartedAt:  time.Now(),
	}

	finished := time.Now().UTC()
	run.Complete(finished)

	if run.Status != "completed" {
		t.Fatalf("expected completed status, got %q", run.Status)
	}
	if run.FinishedAt == nil || !run.FinishedAt.Equal(finished) {
		t.Fatalf("expected finished_at %v, got %v", finished, run.FinishedAt)
	}
}

func TestEventStreamResyncMetadata(t *testing.T) {
	runID := uuid.Must(uuid.NewV4())
	hash := "a3f5"
	event := EventStream{ContentHash: &hash, ResyncRunID: &runID}

	if event.ContentHash == nil || *event.ContentHash != hash {
		t.Fatalf("expected content hash %q, got %v", hash, event.ContentHash)
	}
	if event.ResyncRunID == nil || *event.ResyncRunID != runID {
		t.Fatalf("expected resync run id %s, got %v", runID, event.ResyncRunID)
	}
}
