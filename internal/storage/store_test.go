package storage

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestStoreInsertAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	rec := &RequestRecord{
		ID:       "test-001",
		TunnelID: "tun-abc",
		Method:   "POST",
		Path:     "/webhook",
		Headers:  `{"Content-Type":["application/json"]}`,
		Body:     []byte(`{"event":"test"}`),
	}

	if err := db.Insert(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := db.Get("test-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Method != "POST" {
		t.Errorf("method: got %q, want POST", got.Method)
	}
	if got.Path != "/webhook" {
		t.Errorf("path: got %q, want /webhook", got.Path)
	}
	if string(got.Body) != `{"event":"test"}` {
		t.Errorf("body: got %q", got.Body)
	}
}

func TestStoreUpdateResponse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.Insert(&RequestRecord{
		ID: "test-002", TunnelID: "tun-abc", Method: "POST", Path: "/hook", Headers: "{}",
	})

	err := db.UpdateResponse("test-002", 200, `{"X-Custom":["yes"]}`, []byte("ok"), 42)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := db.Get("test-002")
	if got.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", got.StatusCode)
	}
	if got.DurationMs != 42 {
		t.Errorf("duration: got %d, want 42", got.DurationMs)
	}
}

func TestStoreList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	for i := range 5 {
		db.Insert(&RequestRecord{
			ID: fmt.Sprintf("list-%03d", i), TunnelID: "tun", Method: "GET", Path: "/", Headers: "{}",
		})
	}

	records, err := db.List(3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("count: got %d, want 3", len(records))
	}
}

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}
