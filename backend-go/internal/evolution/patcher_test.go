package evolution

import (
	"testing"
)

func TestGeneratePatchFromDefect(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("patcher-test", "You are a coding assistant."))
	auditor := NewAuditor(s)
	patcher := NewPatcher(s, auditor)

	d := &Defect{
		PromptID: pID,
		Type:     "execution_error",
		Evidence: "timeout handling needed",
		Severity: "high",
	}
	_, err := s.CreateDefect(d)
	if err != nil {
		t.Fatalf("create defect: %v", err)
	}

	patch, err := patcher.GeneratePatchFromAnalysis(d)
	if err != nil {
		t.Fatalf("generate patch: %v", err)
	}
	if patch.PromptID != pID {
		t.Errorf("expected prompt_id %d, got %d", pID, patch.PromptID)
	}
	if patch.NewContent == "" {
		t.Error("expected non-empty new content")
	}
	if patch.NewVersion == "v1" {
		t.Error("expected version bump from v1")
	}

	err = patcher.ApplyPatch(patch, false)
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	p, _ := s.GetPrompt(pID)
	if p.Content != patch.NewContent {
		t.Errorf("patch not applied: content mismatch")
	}
}
