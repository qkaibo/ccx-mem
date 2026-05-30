package evolution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// testRouter creates a Gin engine with evolution API routes and :memory: store.
func testRouter() (*gin.Engine, *Store, *Publisher) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := testStore()

	// Disabled publisher for most API tests (no external calls)
	pub := NewPublisher(store, PublisherConfig{Enabled: false})

	deps := &APIDeps{
		Store:     store,
		Publisher: pub,
		Tracker:   nil,
		Analyzer:  nil,
		Auditor:   nil,
		Patcher:   nil,
	}
	SetupRoutes(&r.RouterGroup, deps)
	return r, store, pub
}

// ========== Skill API ==========

func TestAPICreateSkill(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	body := map[string]string{
		"name":        "api-skill",
		"description": "test",
		"content":     "test content",
		"version":     "v1",
		"category":    "devops",
		"status":      "active",
		"tags":        "api,test",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v2/evolution/skills", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]int64
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] <= 0 {
		t.Errorf("expected positive id, got %d", resp["id"])
	}
}

func TestAPIGetSkill(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	// Create via store
	id, _ := store.CreateSkill(&Skill{
		Name: "get-me", Description: "desc", Content: "cnt",
		Version: "v1", Category: "test", Status: "active",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v2/evolution/skills/"+fmt.Sprintf("%d", id), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var sk Skill
	json.Unmarshal(w.Body.Bytes(), &sk)
	if sk.Name != "get-me" {
		t.Errorf("expected 'get-me', got '%s'", sk.Name)
	}
}

func TestAPIGetSkill_NotFound(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v2/evolution/skills/99999", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIListSkills(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	store.CreateSkill(&Skill{Name: "a", Category: "devops", Status: "active"})
	store.CreateSkill(&Skill{Name: "b", Category: "mlops", Status: "draft"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v2/evolution/skills", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var skills []Skill
	json.Unmarshal(w.Body.Bytes(), &skills)
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	// Filter by category
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/v2/evolution/skills?category=devops", nil)
	r.ServeHTTP(w2, req2)

	var devops []Skill
	json.Unmarshal(w2.Body.Bytes(), &devops)
	if len(devops) != 1 {
		t.Errorf("expected 1 devops, got %d", len(devops))
	}
}

func TestAPIUpdateSkill(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	id, _ := store.CreateSkill(&Skill{Name: "old", Category: "test", Status: "active"})

	body := map[string]string{"name": "new-name", "version": "v2"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v2/evolution/skills/"+fmt.Sprintf("%d", id), bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	sk, _ := store.GetSkill(id)
	if sk.Name != "new-name" {
		t.Errorf("expected 'new-name', got '%s'", sk.Name)
	}
}

func TestAPIDeleteSkill(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	id, _ := store.CreateSkill(&Skill{Name: "del-me", Category: "test", Status: "active"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v2/evolution/skills/"+fmt.Sprintf("%d", id), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	sk, _ := store.GetSkill(id)
	if sk != nil {
		t.Errorf("expected nil after delete")
	}
}

// ========== Shared Learning API ==========

func TestAPIListSharedLearning(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	store.CreateSharedLearning(&SharedLearningRecord{SourceType: "defect", SourceID: 1, Published: true})
	store.CreateSharedLearning(&SharedLearningRecord{SourceType: "patch", SourceID: 2, Published: false})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v2/evolution/shared-learning", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var records []SharedLearningRecord
	json.Unmarshal(w.Body.Bytes(), &records)
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

// ========== Publish API ==========

func TestAPIPublishDefect(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	pID, _ := store.CreatePrompt(newPrompt("pub-defect", "test"))
	dID, _ := store.CreateDefect(&Defect{PromptID: pID, Type: "test", Severity: "low", Evidence: "test evidence"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v2/evolution/publish/defect/"+fmt.Sprintf("%d", dID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPublishDefect_NotFound(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v2/evolution/publish/defect/99999", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPISearchOpenViking_Disabled(t *testing.T) {
	r, store, _ := testRouter()
	defer store.Close()

	body := map[string]string{"query": "test"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v2/evolution/search", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for disabled search, got %d", w.Code)
	}
}
