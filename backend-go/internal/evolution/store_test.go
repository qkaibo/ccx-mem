package evolution

import (
	"testing"
)

func testStore() *Store {
	s, err := NewStore(&StoreConfig{DBPath: ":memory:"})
	if err != nil {
		panic(err)
	}
	return s
}

func newPrompt(name, content string) *Prompt {
	return &Prompt{Name: name, Content: content, Version: "v1"}
}

func TestCreateAndGetPrompt(t *testing.T) {
	s := testStore()
	defer s.Close()

	id, err := s.CreatePrompt(newPrompt("test-1", "You are a helpful assistant."))
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	p, err := s.GetPrompt(id)
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	if p.Name != "test-1" {
		t.Errorf("expected name 'test-1', got '%s'", p.Name)
	}
	if p.Version != "v1" {
		t.Errorf("expected version 'v1', got '%s'", p.Version)
	}
}

func TestUpdatePrompt(t *testing.T) {
	s := testStore()
	defer s.Close()

	id, _ := s.CreatePrompt(newPrompt("test-2", "original"))
	err := s.UpdatePrompt(id, &Prompt{Content: "updated", Version: "v2"})
	if err != nil {
		t.Fatalf("update prompt: %v", err)
	}

	p, _ := s.GetPrompt(id)
	if p.Content != "updated" {
		t.Errorf("expected 'updated', got '%s'", p.Content)
	}
	if p.Version != "v2" {
		t.Errorf("expected 'v2', got '%s'", p.Version)
	}
}

func TestRecordAndGetTraces(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("tracer-test", "system: help"))

	t1 := &ExecutionTrace{
		PromptID:       pID,
		UserID:         "user-a",
		RequestSummary: "some request",
		Success:        true,
		ErrorType:      "",
		LatencyMs:      1500,
	}
	id1, err := s.RecordTrace(t1)
	if err != nil {
		t.Fatalf("record trace: %v", err)
	}
	if id1 <= 0 {
		t.Errorf("expected positive id, got %d", id1)
	}

	traces, err := s.GetRecentTraces(10)
	if err != nil {
		t.Fatalf("get recent traces: %v", err)
	}
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].LatencyMs != 1500 {
		t.Errorf("expected 1500ms, got %d", traces[0].LatencyMs)
	}
}

func TestUnanalyzedTraces(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("unanalyzed-test", "system: test"))
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: true})
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: false, ErrorType: "timeout"})

	traces, err := s.GetUnanalyzedTraces(10)
	if err != nil {
		t.Fatalf("get unanalyzed: %v", err)
	}
	if len(traces) != 2 {
		t.Fatalf("expected 2 unanalyzed, got %d", len(traces))
	}

	ids := []int64{traces[0].ID, traces[1].ID}
	if err := s.MarkTracesAnalyzed(ids); err != nil {
		t.Fatalf("mark analyzed: %v", err)
	}

	traces2, _ := s.GetUnanalyzedTraces(10)
	if len(traces2) != 0 {
		t.Errorf("expected 0 unanalyzed after marking, got %d", len(traces2))
	}
}

func TestCreateAndGetDefect(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("defect-test", "system: x"))

	d := &Defect{
		PromptID: pID,
		Type:     "execution_error",
		Evidence: "repeated timeout",
		Severity: "high",
	}
	id, err := s.CreateDefect(d)
	if err != nil {
		t.Fatalf("create defect: %v", err)
	}

	defect, err := s.GetDefect(id)
	if err != nil {
		t.Fatalf("get defect: %v", err)
	}
	if defect.Type != "execution_error" {
		t.Errorf("expected 'execution_error', got '%s'", defect.Type)
	}
}
