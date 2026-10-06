package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTopologyAndAckReview(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := t.Context()
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

	topology := NewTopology(db)
	entry := TopologyEntry{INN: "7700000001", KPP: "770001001", EDOID: "A", Purpose: "филиал", Author: "admin", UpdatedAt: now}
	if err := topology.Put(ctx, entry); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entry.Purpose = "обособленное подразделение"
	if err := topology.Put(ctx, entry); err != nil {
		t.Fatalf("Put повторно: %v", err)
	}
	all, err := topology.All(ctx)
	if err != nil || len(all) != 1 || all[0].Purpose != "обособленное подразделение" {
		t.Fatalf("All: %+v, %v", all, err)
	}
	if err := topology.Remove(ctx, "7700000001", "770001001", "A"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if all, _ := topology.All(ctx); len(all) != 0 {
		t.Errorf("после Remove осталось %d", len(all))
	}

	ids := NewIdentifiers(db)
	if err := ids.Acknowledge(ctx, "A", "fp", "законно", "admin", now); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	acks, err := ids.Acks(ctx)
	if err != nil {
		t.Fatalf("Acks: %v", err)
	}
	if a := acks["A/fp"]; a.Reason != "законно" || !a.ReviewAt.Equal(now.Add(AckReviewAfter)) {
		t.Errorf("пометка %+v", a)
	}
	if err := ids.Unacknowledge(ctx, "A", "fp"); err != nil {
		t.Fatalf("Unacknowledge: %v", err)
	}
	if acks, _ := ids.Acks(ctx); len(acks) != 0 {
		t.Errorf("после снятия осталось %d пометок", len(acks))
	}
}
