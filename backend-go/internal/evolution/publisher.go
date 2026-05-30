package evolution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// PublisherConfig holds configuration for the OpenViking publisher.
type PublisherConfig struct {
	Endpoint string // OPENVIKING_ENDPOINT
	APIKey   string // OPENVIKING_API_KEY
	Enabled  bool
}

// Publisher publishes evolution findings to OpenViking knowledge base.
type Publisher struct {
	cfg    PublisherConfig
	store  *Store
	client *http.Client
}

// NewPublisher creates a new OpenViking publisher.
func NewPublisher(store *Store, cfg PublisherConfig) *Publisher {
	return &Publisher{
		cfg:    cfg,
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// PublishDefect publishes a defect to OpenViking.
func (p *Publisher) PublishDefect(defect *Defect) (*SharedLearningRecord, error) {
	if !p.cfg.Enabled {
		return nil, nil
	}

	promptName := fmt.Sprintf("prompt-%d", defect.PromptID)
	if defect.PromptID > 0 {
		if prompt, err := p.store.GetPrompt(defect.PromptID); err == nil && prompt != nil {
			promptName = prompt.Name
		}
	}

	content := fmt.Sprintf(`# Evolution Defect: %s

**Type**: %s
**Severity**: %s
**Prompt**: %s
**Discovered**: %s

## Evidence

%s
`,
		defect.Type, defect.Type, defect.Severity, promptName,
		time.Now().Format("2006-01-02 15:04:05"),
		defect.Evidence,
	)

	targetURI := fmt.Sprintf("viking://resources/learnings/evolution/defects/%d/%s-%d.md",
		defect.PromptID, defect.Type, defect.ID)

	uri, err := p.uploadContent(content, targetURI, fmt.Sprintf("Auto-published defect #%d (%s)", defect.ID, defect.Type))
	if err != nil {
		log.Printf("[Evolution-Publisher] 发布缺陷失败: id=%d, err=%v", defect.ID, err)
		rec := &SharedLearningRecord{
			SourceType:   "defect",
			SourceID:     defect.ID,
			TargetURI:    targetURI,
			Published:    false,
			ErrorMessage: err.Error(),
			CreatedAt:    time.Now(),
		}
		p.store.CreateSharedLearning(rec)
		return rec, nil
	}

	rec := &SharedLearningRecord{
		SourceType:  "defect",
		SourceID:    defect.ID,
		TargetURI:   uri,
		Published:   true,
		PublishedAt: timePtr(time.Now()),
		CreatedAt:   time.Now(),
	}
	id, err := p.store.CreateSharedLearning(rec)
	if err != nil {
		log.Printf("[Evolution-Publisher] 记录共享学习失败: %v", err)
	}
	rec.ID = id
	log.Printf("[Evolution-Publisher] 缺陷已发布: id=%d -> %s", defect.ID, uri)
	return rec, nil
}

// PublishPatch publishes a patcher result to OpenViking.
// It fetches the old version and content from the store before the patch was applied.
func (p *Publisher) PublishPatch(patch *Patch) (*SharedLearningRecord, error) {
	if !p.cfg.Enabled {
		return nil, nil
	}

	promptName := fmt.Sprintf("prompt-%d", patch.PromptID)
	oldVersion := ""
	oldContent := ""
	if patch.PromptID > 0 {
		if prompt, err := p.store.GetPrompt(patch.PromptID); err == nil && prompt != nil {
			promptName = prompt.Name
			oldVersion = prompt.Version
			oldContent = prompt.Content
		}
	}

	content := fmt.Sprintf(`# Evolution Patch: %s

**Prompt**: %s
**Version**: %s → %s
**Applied**: %s
**Reason**: %s

## Old Content

`+"```"+`
%s
`+"```"+`

## New Content

`+"```"+`
%s
`+"```"+``,
		promptName, promptName, oldVersion, patch.NewVersion,
		patch.CreatedAt.Format("2006-01-02 15:04:05"),
		patch.Reason,
		oldContent,
		patch.NewContent,
	)

	targetURI := fmt.Sprintf("viking://resources/learnings/evolution/patches/%d/v%s-%s.md",
		patch.PromptID, patch.NewVersion, formatTimestamp(time.Now()))

	uri, err := p.uploadContent(content, targetURI, fmt.Sprintf("Auto-published patch for %s v%s", promptName, patch.NewVersion))
	if err != nil {
		log.Printf("[Evolution-Publisher] 发布补丁失败: prompt=%d, err=%v", patch.PromptID, err)
		return nil, err
	}

	rec := &SharedLearningRecord{
		SourceType:  "patch",
		SourceID:    patch.PromptID,
		TargetURI:   uri,
		Published:   true,
		PublishedAt: timePtr(time.Now()),
		CreatedAt:   time.Now(),
	}
	p.store.CreateSharedLearning(rec)
	log.Printf("[Evolution-Publisher] 补丁已发布: prompt=%d -> %s", patch.PromptID, uri)
	return rec, nil
}

// PublishAuditResult publishes an audit result to OpenViking.
func (p *Publisher) PublishAuditResult(audit *AuditLog) (*SharedLearningRecord, error) {
	if !p.cfg.Enabled {
		return nil, nil
	}

	promptName := fmt.Sprintf("prompt-%d", audit.PromptID)
	if audit.PromptID > 0 {
		if prompt, err := p.store.GetPrompt(audit.PromptID); err == nil && prompt != nil {
			promptName = prompt.Name
		}
	}

	status := "PASS"
	if !audit.Passed {
		status = "FAIL"
	}

	content := fmt.Sprintf(`# Evolution Audit: %s

**Prompt**: %s
**Status**: %s
**Rules**: %d/%d passed (%d failed)
**Checked**: %s

## Violations

%s
`,
		promptName, promptName, status, audit.RulesPassed, audit.RulesChecked, audit.RulesFailed,
		audit.CreatedAt.Format("2006-01-02 15:04:05"),
		audit.Violations,
	)

	targetURI := fmt.Sprintf("viking://resources/learnings/evolution/audits/%d/%s.md",
		audit.PromptID, formatTimestamp(audit.CreatedAt))

	uri, err := p.uploadContent(content, targetURI, fmt.Sprintf("Auto-published audit for %s (%s)", promptName, status))
	if err != nil {
		log.Printf("[Evolution-Publisher] 发布审计结果失败: prompt=%d, err=%v", audit.PromptID, err)
		return nil, err
	}

	rec := &SharedLearningRecord{
		SourceType:  "audit",
		SourceID:    audit.ID,
		TargetURI:   uri,
		Published:   true,
		PublishedAt: timePtr(time.Now()),
		CreatedAt:   time.Now(),
	}
	p.store.CreateSharedLearning(rec)
	return rec, nil
}

// publishResult represents the response from OpenViking upload.
type publishResult struct {
	RootURI   string `json:"root_uri"`
	Resources []struct {
		URI string `json:"uri"`
	} `json:"resources"`
	Status string `json:"status"`
}

// uploadContent uploads content to OpenViking via temp_upload + add_resource.
func (p *Publisher) uploadContent(content, targetURI, reason string) (string, error) {
	// Step 1: temp_upload
	uploadURL := p.cfg.Endpoint + "/api/v1/resources/temp_upload"
	body := bytes.NewBufferString(content)

	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "text/markdown")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("temp upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("temp upload status %d", resp.StatusCode)
	}

	var uploadResp struct {
		TempFileID string `json:"temp_file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return "", fmt.Errorf("decode temp upload response: %w", err)
	}
	if uploadResp.TempFileID == "" {
		return "", fmt.Errorf("temp upload returned empty temp_file_id")
	}

	// Step 2: add_resource
	addURL := p.cfg.Endpoint + "/api/v1/resources"
	addPayload := map[string]interface{}{
		"temp_file_id": uploadResp.TempFileID,
		"to":           targetURI,
		"reason":       reason,
		"instruction":  "Extract technical findings about evolution defects, patches, and audit results.",
		"wait":         true,
		"timeout":      30,
	}
	addBody, _ := json.Marshal(addPayload)

	req2, err := http.NewRequest("POST", addURL, bytes.NewBuffer(addBody))
	if err != nil {
		return "", fmt.Errorf("create add resource request: %w", err)
	}
	req2.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := p.client.Do(req2)
	if err != nil {
		return "", fmt.Errorf("add resource request: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("add resource status %d", resp2.StatusCode)
	}

	var result publishResult
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode add resource response: %w", err)
	}

	// Return the first resource URI or root URI
	if len(result.Resources) > 0 && result.Resources[0].URI != "" {
		return result.Resources[0].URI, nil
	}
	if result.RootURI != "" {
		return result.RootURI, nil
	}
	return targetURI, nil
}

// SearchOpenViking searches the OpenViking knowledge base for evolution-related content.
func (p *Publisher) SearchOpenViking(query, targetURI string, limit int) (string, error) {
	if !p.cfg.Enabled || p.cfg.Endpoint == "" {
		return "", fmt.Errorf("publisher not configured")
	}

	if limit <= 0 {
		limit = 5
	}

	searchURL := p.cfg.Endpoint + "/api/v1/search/find"
	payload := map[string]interface{}{
		"query":           query,
		"limit":           limit,
		"score_threshold": 0.3,
	}
	if targetURI != "" {
		payload["target_uri"] = targetURI
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", searchURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("create search request: %w", err)
	}
	req.Header.Set("X-API-Key", p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search status %d", resp.StatusCode)
	}

	var result bytes.Buffer
	result.ReadFrom(resp.Body)
	return result.String(), nil
}

// timePtr returns a pointer to the given time.
func timePtr(t time.Time) *time.Time {
	return &t
}

// formatTimestamp formats a time as a compact string safe for filenames.
func formatTimestamp(t time.Time) string {
	return strings.ReplaceAll(t.Format("20060102_150405"), " ", "_")
}
