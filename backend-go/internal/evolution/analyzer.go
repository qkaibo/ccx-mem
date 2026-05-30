package evolution

import (
	"fmt"
	"strings"
)

// Analyzer examines execution traces to detect defects.
type Analyzer struct {
	Store     *Store
	MaxTraces int // 每次分析最大轨迹数（默认 100）
}

// Edge case thresholds.
const (
	minLatencyForOptimization = 3000 // 3s
	maxOccurrenceForDefect    = 3
)

// NewAnalyzer creates a new analyzer.
func NewAnalyzer(store *Store) *Analyzer {
	return &Analyzer{Store: store}
}

// AnalyzeResult holds the results of analysis.
type AnalyzeResult struct {
	DefectsCreated int      `json:"defects_created"`
	TracesAnalyzed int      `json:"traces_analyzed"`
	DefectIDs      []int64  `json:"defect_ids"`
	Summary        string   `json:"summary"`
}

// Analyze examines unanalyzed traces and creates defects.
func (a *Analyzer) Analyze() (*AnalyzeResult, error) {
	result := &AnalyzeResult{}

	limit := a.MaxTraces
	if limit <= 0 {
		limit = 100
	}

	traces, err := a.Store.GetUnanalyzedTraces(limit)
	if err != nil {
		return nil, fmt.Errorf("get unanalyzed traces: %w", err)
	}
	if len(traces) == 0 {
		result.Summary = "no unanalyzed traces"
		return result, nil
	}

	analyzedIDs := make([]int64, 0, len(traces))
	errorCounts := make(map[string]int)
	errorDefectCreated := make(map[string]bool)

	for _, t := range traces {
		analyzedIDs = append(analyzedIDs, t.ID)
		result.TracesAnalyzed++

		// Execution error detection: repeated errors
		if !t.Success && t.ErrorType != "" {
			errorCounts[t.ErrorType]++
			if errorCounts[t.ErrorType] >= maxOccurrenceForDefect && !errorDefectCreated[t.ErrorType] {
				d := &Defect{
					PromptID: t.PromptID,
					Type:     "execution_error",
					Evidence: fmt.Sprintf("repeated error type '%s' (%d occurrences)", t.ErrorType, errorCounts[t.ErrorType]),
					Severity: "high",
				}
				id, err := a.Store.CreateDefect(d)
				if err != nil {
					return nil, fmt.Errorf("create execution_error defect: %w", err)
				}
				result.DefectIDs = append(result.DefectIDs, id)
				result.DefectsCreated++
				errorDefectCreated[t.ErrorType] = true
			}
		}

		// Optimization: high latency
		if t.LatencyMs >= minLatencyForOptimization && t.Success {
			d := &Defect{
				PromptID: t.PromptID,
				Type:     "optimization",
				Evidence: fmt.Sprintf("high latency %dms in request: '%s'", t.LatencyMs, truncate(t.RequestSummary, 80)),
				Severity: "medium",
			}
			id, err := a.Store.CreateDefect(d)
			if err != nil {
				return nil, fmt.Errorf("create optimization defect: %w", err)
			}
			result.DefectIDs = append(result.DefectIDs, id)
			result.DefectsCreated++
		}

		// Discovery: patterns in request summary
		words := strings.Fields(strings.ToLower(t.RequestSummary))
		if len(words) > 5 && containsNewPattern(words) {
			d := &Defect{
				PromptID: t.PromptID,
				Type:     "discovery",
				Evidence: fmt.Sprintf("new pattern detected in request: '%s'", truncate(t.RequestSummary, 80)),
				Severity: "low",
			}
			id, err := a.Store.CreateDefect(d)
			if err != nil {
				return nil, fmt.Errorf("create discovery defect: %w", err)
			}
			result.DefectIDs = append(result.DefectIDs, id)
			result.DefectsCreated++
		}
	}

	// Mark traces as analyzed
	if err := a.Store.MarkTracesAnalyzed(analyzedIDs); err != nil {
		return nil, fmt.Errorf("mark traces analyzed: %w", err)
	}

	if result.DefectsCreated > 0 {
		result.Summary = fmt.Sprintf("analyzed %d traces, created %d defects", result.TracesAnalyzed, result.DefectsCreated)
	} else {
		result.Summary = fmt.Sprintf("analyzed %d traces, no defects found", result.TracesAnalyzed)
	}
	return result, nil
}

func containsNewPattern(words []string) bool {
	// Heuristic: if the request contains unique technical terms that indicate a new use case
	technicalTerms := []string{"deploy", "migrate", "refactor", "upgrade", "scale", "backup", "restore", "optimize", "debug", "profile", "benchmark", "configure"}
	for _, term := range technicalTerms {
		for _, w := range words {
			if w == term {
				return true
			}
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
