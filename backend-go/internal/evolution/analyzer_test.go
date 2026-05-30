package evolution

import (
	"testing"
)

func TestAnalyzerEmptyStore(t *testing.T) {
	s := testStore()
	defer s.Close()

	a := NewAnalyzer(s)
	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DefectsCreated != 0 {
		t.Errorf("expected 0 defects from empty store, got %d", result.DefectsCreated)
	}
}

func TestAnalyzerWithTraces(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("analyze-test", "system: help"))

	// Normal successful trace
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: true, LatencyMs: 500})

	// High latency trace → should trigger optimization defect
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: true, LatencyMs: 5000, RequestSummary: "debug the system"})

	// Repeated errors → should trigger execution_error defect
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: false, ErrorType: "timeout"})
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: false, ErrorType: "timeout"})
	s.RecordTrace(&ExecutionTrace{PromptID: pID, UserID: "u1", Success: false, ErrorType: "timeout"})

	a := NewAnalyzer(s)
	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.DefectsCreated < 2 {
		t.Errorf("expected at least 2 defects, got %d (summary: %s)", result.DefectsCreated, result.Summary)
	}
	if result.TracesAnalyzed != 5 {
		t.Errorf("expected 5 traces analyzed, got %d", result.TracesAnalyzed)
	}

	unanalyzed, _ := s.GetUnanalyzedTraces(10)
	if len(unanalyzed) != 0 {
		t.Errorf("expected 0 unanalyzed after analysis, got %d", len(unanalyzed))
	}
}
