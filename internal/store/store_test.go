package store

import (
	"testing"
)

func TestLogUpdateAndLastUpdateByTarget(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.LogUpdate("stack", "mystack", "update"); err != nil {
		t.Fatalf("LogUpdate: %v", err)
	}
	if err := s.LogUpdate("container", "myapp", "save_image"); err != nil {
		t.Fatalf("LogUpdate: %v", err)
	}
	if err := s.LogUpdate("stack", "mystack", "save_and_update"); err != nil {
		t.Fatalf("LogUpdate: %v", err)
	}

	byTarget, err := s.LastUpdateByTarget()
	if err != nil {
		t.Fatalf("LastUpdateByTarget: %v", err)
	}
	if len(byTarget) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(byTarget), byTarget)
	}
	if _, ok := byTarget["mystack"]; !ok {
		t.Error("mystack missing")
	}
	if _, ok := byTarget["myapp"]; !ok {
		t.Error("myapp missing")
	}
}

func TestOpenEmptyDB(t *testing.T) {
	s, err := Open(t.TempDir() + "/sub/history.db")
	if err != nil {
		t.Fatalf("Open with missing parent dir: %v", err)
	}
	defer s.Close()

	byTarget, err := s.LastUpdateByTarget()
	if err != nil {
		t.Fatalf("LastUpdateByTarget on empty db: %v", err)
	}
	if len(byTarget) != 0 {
		t.Errorf("expected empty map, got %v", byTarget)
	}
}
