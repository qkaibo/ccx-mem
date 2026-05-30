package evolution

import (
	"testing"
)

func TestAuditorPass(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("audit-pass", "You are a helpful assistant."))
	auditor := NewAuditor(s)

	result, err := auditor.Audit(pID, "You are a helpful assistant.\nProvide detailed answers.")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !result.Passed {
		t.Logf("violations: %v", result.Violations)
	}
	if !result.Passed {
		t.Error("expected audit to pass for well-structured prompt")
	}
}
