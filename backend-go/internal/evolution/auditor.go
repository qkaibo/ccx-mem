package evolution

import (
	"fmt"
	"strings"
)

// AuditRule represents a single audit rule.
type AuditRule struct {
	Name        string
	Description string
	CheckFn     func(content string) (passed bool, detail string)
}

// DefaultAuditRules returns the 9 standard audit rules.
func DefaultAuditRules() []AuditRule {
	return []AuditRule{
		{
			Name:        "hardcoded_literals",
			Description: "No hardcoded IPs, ports, secrets or magic numbers",
			CheckFn: func(content string) (bool, string) {
				lower := strings.ToLower(content)
				bad := []string{"password=", "secret=", "api_key=", "token=", "0.0.0.0", "127.0.0.1", "localhost:8080"}
				for _, b := range bad {
					if strings.Contains(lower, b) {
						return false, fmt.Sprintf("contains hardcoded literal: '%s'", b)
					}
				}
				return true, ""
			},
		},
		{
			Name:        "silent_bypass",
			Description: "No silent error swallowing or empty catch blocks",
			CheckFn: func(content string) (bool, string) {
				lower := strings.ToLower(content)
				suspects := []string{"catch {}", "except:", "_ = err", "// ignore error", "// ignore err", "// nolint:errcheck"}
				for _, s := range suspects {
					if strings.Contains(lower, s) {
						return false, fmt.Sprintf("potential silent error bypass: '%s'", s)
					}
				}
				return true, ""
			},
		},
		{
			Name:        "untraceable_assertions",
			Description: "No bare assertions without descriptive messages",
			CheckFn: func(content string) (bool, string) {
				lower := strings.ToLower(content)
				if strings.Contains(lower, "assert true") || strings.Contains(lower, "assert false") ||
					strings.Contains(lower, "assert.equal") || strings.Contains(lower, "assert.equal") {
					return false, "bare assertion without descriptive message"
				}
				return true, ""
			},
		},
		{
			Name:        "framework_borrowing",
			Description: "No confusing framework-specific patterns that don't apply",
			CheckFn: func(content string) (bool, string) {
				lower := strings.ToLower(content)
				suspects := []string{"react.component", "vue.component", "django.", "flask.", "middleware("}
				for _, s := range suspects {
					if strings.Contains(lower, s) {
						return false, fmt.Sprintf("framework-specific pattern: '%s'", s)
					}
				}
				return true, ""
			},
		},
		{
			Name:        "redundant_logic",
			Description: "No redundant or duplicate instructions",
			CheckFn: func(content string) (bool, string) {
				lines := strings.Split(content, "\n")
				seen := make(map[string]bool)
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if len(trimmed) > 20 {
						normalized := strings.ToLower(trimmed)
						if seen[normalized] {
							return false, fmt.Sprintf("duplicate line: '%s'", truncate(trimmed, 60))
						}
						seen[normalized] = true
					}
				}
				return true, ""
			},
		},
		{
			Name:        "cross_reference",
			Description: "No dangling cross-references to non-existent resources",
			CheckFn: func(content string) (bool, string) {
				// Check for common broken reference patterns
				if strings.Contains(content, "see above") || strings.Contains(content, "as mentioned below") {
					return false, "contains vague cross-reference (above/below)"
				}
				return true, ""
			},
		},
		{
			Name:        "under_abstraction",
			Description: "No overly generic instructions that should be specific",
			CheckFn: func(content string) (bool, string) {
				lower := strings.ToLower(content)
				vague := []string{"handle it", "deal with it", "figure it out", "do the right thing", "use best practices"}
				for _, v := range vague {
					if strings.Contains(lower, v) {
						return false, fmt.Sprintf("vague instruction: '%s'", v)
					}
				}
				return true, ""
			},
		},
		{
			Name:        "main_op_promotion",
			Description: "Main operations should be explicit, not buried in details",
			CheckFn: func(content string) (bool, string) {
				// The main operation should be at the beginning, not >200 chars in
				if len(content) > 300 {
					head := strings.ToLower(content[:200])
					if !strings.Contains(head, "you are") && !strings.Contains(head, "your task") &&
						!strings.Contains(head, "your goal") && !strings.Contains(head, "your role") {
						return false, "main operation not declared in first 200 chars"
					}
				}
				return true, ""
			},
		},
		{
			Name:        "script_inflation",
			Description: "No tool scripts over 2000 chars without justification",
			CheckFn: func(content string) (bool, string) {
				if len(content) > 2000 {
					// Check if it's a code block (which is fine)
					if strings.Count(content, "```") >= 2 {
						return true, "" // Code blocks are OK
					}
					return false, fmt.Sprintf("prompt content too long (%d chars) without code blocks", len(content))
				}
				return true, ""
			},
		},
	}
}

// Auditor runs audit rules against prompt content.
type Auditor struct {
	Store *Store
	Rules []AuditRule
}

// NewAuditor creates a new auditor with default rules.
func NewAuditor(store *Store) *Auditor {
	return &Auditor{
		Store: store,
		Rules: DefaultAuditRules(),
	}
}

// AuditResult holds the result of auditing a prompt.
type AuditResult struct {
	PromptID     int64    `json:"prompt_id"`
	RulesChecked int      `json:"rules_checked"`
	RulesPassed  int      `json:"rules_passed"`
	RulesFailed  int      `json:"rules_failed"`
	Passed       bool     `json:"passed"`
	Violations   []string `json:"violations"`
}

// Audit runs all rules against a prompt's content.
func (a *Auditor) Audit(promptID int64, content string) (*AuditResult, error) {
	result := &AuditResult{
		PromptID:     promptID,
		RulesChecked: len(a.Rules),
	}

	for _, rule := range a.Rules {
		passed, detail := rule.CheckFn(content)
		if passed {
			result.RulesPassed++
		} else {
			result.RulesFailed++
			result.Violations = append(result.Violations, fmt.Sprintf("[%s] %s", rule.Name, detail))
		}
	}

	result.Passed = result.RulesFailed == 0

	// Save audit log
	violationsStr := strings.Join(result.Violations, "; ")
	log := &AuditLog{
		PromptID:     promptID,
		RulesChecked: result.RulesChecked,
		RulesPassed:  result.RulesPassed,
		RulesFailed:  result.RulesFailed,
		Violations:   violationsStr,
		Passed:       result.Passed,
	}
	if _, err := a.Store.CreateAuditLog(log); err != nil {
		return nil, fmt.Errorf("save audit log: %w", err)
	}

	return result, nil
}
