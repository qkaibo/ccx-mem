package evolution

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testPublisher creates a Publisher pointed at a test HTTP server.
// The handler should respond to temp_upload and add_resource endpoints.
func testPublisher(handler http.Handler) (*Publisher, *Store, *httptest.Server) {
	srv := httptest.NewServer(handler)
	store := testStore()
	p := NewPublisher(store, PublisherConfig{
		Endpoint: srv.URL,
		APIKey:   "test-key",
		Enabled:  true,
	})
	return p, store, srv
}

// ========== Disabled Publisher ==========

func TestPublisher_Disabled(t *testing.T) {
	store := testStore()
	defer store.Close()

	p := NewPublisher(store, PublisherConfig{Enabled: false})

	defect := &Defect{ID: 1, PromptID: 1, Type: "test"}
	rec, err := p.PublishDefect(defect)
	if err != nil {
		t.Fatalf("disabled PublishDefect should not error: %v", err)
	}
	if rec != nil {
		t.Errorf("disabled publisher should return nil record, got %+v", rec)
	}

	patch := &Patch{PromptID: 1, NewVersion: "v2", NewContent: "x", Reason: "test"}
	rec2, err := p.PublishPatch(patch)
	if err != nil {
		t.Fatalf("disabled PublishPatch should not error: %v", err)
	}
	if rec2 != nil {
		t.Errorf("disabled publisher should return nil for patch")
	}

	audit := &AuditLog{ID: 1, PromptID: 1, RulesChecked: 1, RulesPassed: 1}
	rec3, err := p.PublishAuditResult(audit)
	if err != nil {
		t.Fatalf("disabled PublishAuditResult should not error: %v", err)
	}
	if rec3 != nil {
		t.Errorf("disabled publisher should return nil for audit")
	}
}

// ========== PublishDefect with Mock Server ==========

func TestPublishDefect_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/resources/temp_upload" {
			json.NewEncoder(w).Encode(map[string]string{"temp_file_id": "temp-123"})
			return
		}
		if r.URL.Path == "/api/v1/resources" {
			resp := publishResult{
				Status:  "ok",
				RootURI: "viking://resources/learnings/test.md",
				Resources: []struct {
					URI string `json:"uri"`
				}{{URI: "viking://resources/learnings/test.md"}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	p, store, srv := testPublisher(handler)
	defer srv.Close()
	defer store.Close()

	// Create a prompt first (PublishDefect fetches prompt name)
	pID, _ := store.CreatePrompt(newPrompt("test-prompt", "test content"))

	defect := &Defect{ID: 1, PromptID: pID, Type: "execution_error", Severity: "high", Evidence: "timeout"}
	rec, err := p.PublishDefect(defect)
	if err != nil {
		t.Fatalf("publish defect: %v", err)
	}
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if !rec.Published {
		t.Errorf("expected published=true")
	}
	if rec.SourceType != "defect" {
		t.Errorf("expected source_type 'defect', got '%s'", rec.SourceType)
	}
	if rec.SourceID != 1 {
		t.Errorf("expected source_id 1, got %d", rec.SourceID)
	}
	if rec.TargetURI == "" {
		t.Errorf("expected non-empty target_uri")
	}

	// Verify SharedLearningRecord was written
	records, _ := store.ListSharedLearning(10)
	if len(records) != 1 {
		t.Fatalf("expected 1 SharedLearningRecord, got %d", len(records))
	}
}

func TestPublishDefect_HTTPError(t *testing.T) {
	// This test verifies that when temp_upload fails, the record is still
	// created with published=false and error_message set
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	p, store, srv := testPublisher(handler)
	defer srv.Close()
	defer store.Close()

	pID, _ := store.CreatePrompt(newPrompt("test", "x"))
	defect := &Defect{ID: 1, PromptID: pID, Type: "execution_error", Severity: "high", Evidence: "timeout"}

	rec, err := p.PublishDefect(defect)
	if err != nil {
		t.Fatalf("publish defect should not error even on HTTP failure: %v", err)
	}
	if rec.Published {
		t.Errorf("expected published=false on HTTP error")
	}
	if rec.ErrorMessage == "" {
		t.Errorf("expected error_message on HTTP failure")
	}

	// Still created a SharedLearningRecord
	records, _ := store.ListSharedLearning(10)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Published {
		t.Errorf("expected published=false in stored record")
	}
}

// ========== PublishPatch ==========

func TestPublishPatch_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/resources/temp_upload" {
			json.NewEncoder(w).Encode(map[string]string{"temp_file_id": "temp-456"})
			return
		}
		if r.URL.Path == "/api/v1/resources" {
			json.NewEncoder(w).Encode(publishResult{
				Status:  "ok",
				RootURI: "viking://resources/learnings/patches/test.md",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	p, store, srv := testPublisher(handler)
	defer srv.Close()
	defer store.Close()

	pID, _ := store.CreatePrompt(newPrompt("patchable", "old content"))

	patch := &Patch{
		PromptID:   pID,
		NewVersion: "v2",
		NewContent: "new improved content",
		Reason:     "fix bug",
		CreatedAt:  time.Now(),
	}
	rec, err := p.PublishPatch(patch)
	if err != nil {
		t.Fatalf("publish patch: %v", err)
	}
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if !rec.Published {
		t.Errorf("expected published=true")
	}
	if rec.SourceType != "patch" {
		t.Errorf("expected source_type 'patch', got '%s'", rec.SourceType)
	}
}

// ========== PublishAudit ==========

func TestPublishAudit_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/resources/temp_upload" {
			json.NewEncoder(w).Encode(map[string]string{"temp_file_id": "temp-789"})
			return
		}
		if r.URL.Path == "/api/v1/resources" {
			json.NewEncoder(w).Encode(publishResult{
				Status:  "ok",
				RootURI: "viking://resources/learnings/audits/test.md",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	p, store, srv := testPublisher(handler)
	defer srv.Close()
	defer store.Close()

	pID, _ := store.CreatePrompt(newPrompt("auditable", "test"))

	audit := &AuditLog{
		ID:           1,
		PromptID:     pID,
		RulesChecked: 5,
		RulesPassed:  3,
		RulesFailed:  2,
		Violations:   "issue 1",
		Passed:       false,
		CreatedAt:    time.Now(),
	}
	rec, err := p.PublishAuditResult(audit)
	if err != nil {
		t.Fatalf("publish audit: %v", err)
	}
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if !rec.Published {
		t.Errorf("expected published=true")
	}
	if rec.SourceType != "audit" {
		t.Errorf("expected source_type 'audit', got '%s'", rec.SourceType)
	}
	if rec.SourceID != 1 {
		t.Errorf("expected source_id 1, got %d", rec.SourceID)
	}
}

func TestPublishAudit_Disabled(t *testing.T) {
	store := testStore()
	defer store.Close()

	p := NewPublisher(store, PublisherConfig{Enabled: false})
	audit := &AuditLog{ID: 1, PromptID: 1}

	rec, err := p.PublishAuditResult(audit)
	if err != nil {
		t.Fatalf("disabled publish should not error: %v", err)
	}
	if rec != nil {
		t.Errorf("disabled should return nil, got %+v", rec)
	}
}

// ========== SearchOpenViking ==========

func TestSearchOpenViking_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results": [{"uri": "viking://test.md", "score": 0.95}]}`))
	})

	p, store, srv := testPublisher(handler)
	defer srv.Close()
	defer store.Close()

	result, err := p.SearchOpenViking("test query", "", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty result")
	}
}

func TestSearchOpenViking_Disabled(t *testing.T) {
	p := NewPublisher(nil, PublisherConfig{Enabled: false, Endpoint: ""})
	_, err := p.SearchOpenViking("test", "", 5)
	if err == nil {
		t.Errorf("expected error for disabled search")
	}
}

func TestSearchOpenViking_HTTPError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	p, store, srv := testPublisher(handler)
	defer srv.Close()
	defer store.Close()

	_, err := p.SearchOpenViking("test", "", 5)
	if err == nil {
		t.Errorf("expected error on HTTP 500")
	}
}
