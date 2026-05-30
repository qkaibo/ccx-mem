package evolution

import (
	"fmt"
	"time"
)

// Patch represents a suggested change to a prompt.
type Patch struct {
	PromptID    int64     `json:"prompt_id"`
	NewVersion  string    `json:"new_version"`
	NewContent  string    `json:"new_content"`
	Reason      string    `json:"reason"`
	Applied     bool      `json:"applied"`
	AuditPassed bool      `json:"audit_passed"`
	CreatedAt   time.Time `json:"created_at"`
}

// Patcher generates and applies patches for prompts.
type Patcher struct {
	Store   *Store
	Auditor *Auditor
}

// NewPatcher creates a new patcher.
func NewPatcher(store *Store, auditor *Auditor) *Patcher {
	return &Patcher{
		Store:   store,
		Auditor: auditor,
	}
}

// GeneratePatchFromAnalysis creates a patch from a defect.
// This is heuristic-based: it uses the defect evidence to suggest content changes.
func (p *Patcher) GeneratePatchFromAnalysis(defect *Defect) (*Patch, error) {
	// Get the associated prompt
	prompt, err := p.Store.GetPrompt(defect.PromptID)
	if err != nil {
		return nil, fmt.Errorf("get prompt for patching: %w", err)
	}

	patch := &Patch{
		PromptID: defect.PromptID,
		Reason:   fmt.Sprintf("defect: [%s] %s", defect.Type, defect.Evidence),
	}

	// Generate new content based on defect type
	switch defect.Type {
	case "discovery":
		// New pattern discovered — add a note about it
		patch.NewContent = prompt.Content + fmt.Sprintf("\n\n[Auto-discovery %s] Handle: %s",
			time.Now().Format("2006-01-02"), defect.Evidence)
		patch.NewVersion = bumpVersion(prompt.Version)
	case "optimization":
		// Optimization hint — add performance note
		patch.NewContent = prompt.Content + fmt.Sprintf("\n\n[Optimization hint] Latency improvement opportunity: %s",
			defect.Evidence)
		patch.NewVersion = bumpVersion(prompt.Version)
	case "skill_defect", "execution_error":
		// Skill defect — add explicit error handling instruction
		patch.NewContent = prompt.Content + fmt.Sprintf("\n\n[Error handling augmentation %s] Mitigation: %s",
			time.Now().Format("2006-01-02"), defect.Evidence)
		patch.NewVersion = bumpVersion(prompt.Version)
	default:
		return nil, fmt.Errorf("unknown defect type: %s", defect.Type)
	}

	return patch, nil
}

// ApplyPatch applies a patch after auditing it (if audit is required).
func (p *Patcher) ApplyPatch(patch *Patch, requireAudit bool) error {
	if requireAudit {
		auditResult, err := p.Auditor.Audit(patch.PromptID, patch.NewContent)
		if err != nil {
			return fmt.Errorf("audit before patch: %w", err)
		}
		patch.AuditPassed = auditResult.Passed
		if !auditResult.Passed {
			return fmt.Errorf("audit failed: %d violations", len(auditResult.Violations))
		}
	}

	// Apply the patch
	if err := p.Store.UpdatePrompt(patch.PromptID, &Prompt{
		Content: patch.NewContent,
		Version: patch.NewVersion,
	}); err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}
	patch.Applied = true
	return nil
}

func bumpVersion(v string) string {
	// Simple version bump: v1 -> v2, v2 -> v3, etc.
	var num int
	if _, err := fmt.Sscanf(v, "v%d", &num); err == nil {
		return fmt.Sprintf("v%d", num+1)
	}
	return fmt.Sprintf("%s.1", v)
}
