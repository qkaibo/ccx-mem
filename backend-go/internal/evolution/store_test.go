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

// ========== Phase 4: Skill CRUD ==========

func newSkill(name, category string) *Skill {
	return &Skill{
		Name:        name,
		Description: "A test skill",
		Content:     "This is a test skill content",
		Version:     "v1",
		Author:      "test-author",
		Tags:        "testing,unit",
		Category:    category,
		Status:      "active",
	}
}

func TestCreateAndGetSkill(t *testing.T) {
	s := testStore()
	defer s.Close()

	id, err := s.CreateSkill(newSkill("test-skill", "devops"))
	if err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	sk, err := s.GetSkill(id)
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if sk.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", sk.Name)
	}
	if sk.Category != "devops" {
		t.Errorf("expected category 'devops', got '%s'", sk.Category)
	}
	if sk.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", sk.Status)
	}
	if sk.Version != "v1" {
		t.Errorf("expected version 'v1', got '%s'", sk.Version)
	}
	if sk.Tags != "testing,unit" {
		t.Errorf("expected tags 'testing,unit', got '%s'", sk.Tags)
	}
}

func TestGetSkill_NotFound(t *testing.T) {
	s := testStore()
	defer s.Close()

	sk, err := s.GetSkill(99999)
	if err != nil {
		t.Fatalf("get skill error: %v", err)
	}
	if sk != nil {
		t.Errorf("expected nil for non-existent skill, got %+v", sk)
	}
}

func TestListSkills(t *testing.T) {
	s := testStore()
	defer s.Close()

	s.CreateSkill(newSkill("skill-a", "devops"))
	s.CreateSkill(newSkill("skill-b", "devops"))
	s.CreateSkill(newSkill("skill-c", "mlops"))

	// List all
	all, err := s.ListSkills("", "", 50, 0)
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 skills, got %d", len(all))
	}

	// Filter by category
	devops, err := s.ListSkills("devops", "", 50, 0)
	if err != nil {
		t.Fatalf("list skills by category: %v", err)
	}
	if len(devops) != 2 {
		t.Errorf("expected 2 devops skills, got %d", len(devops))
	}

	// Filter by status
	active, err := s.ListSkills("", "active", 50, 0)
	if err != nil {
		t.Fatalf("list skills by status: %v", err)
	}
	if len(active) != 3 {
		t.Errorf("expected 3 active skills, got %d", len(active))
	}

	// Filter by both
	both, err := s.ListSkills("mlops", "active", 50, 0)
	if err != nil {
		t.Fatalf("list skills by both: %v", err)
	}
	if len(both) != 1 {
		t.Errorf("expected 1 mlops+active skill, got %d", len(both))
	}

	// Pagination
	limited, err := s.ListSkills("", "", 2, 0)
	if err != nil {
		t.Fatalf("list skills with limit: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 skills with limit, got %d", len(limited))
	}
}

func TestUpdateSkill(t *testing.T) {
	s := testStore()
	defer s.Close()

	id, _ := s.CreateSkill(newSkill("update-me", "devops"))

	err := s.UpdateSkill(id, &Skill{
		Name:    "updated-skill",
		Content: "new content",
		Version: "v2",
		Status:  "draft",
	})
	if err != nil {
		t.Fatalf("update skill: %v", err)
	}

	sk, _ := s.GetSkill(id)
	if sk.Name != "updated-skill" {
		t.Errorf("expected name 'updated-skill', got '%s'", sk.Name)
	}
	if sk.Content != "new content" {
		t.Errorf("expected 'new content', got '%s'", sk.Content)
	}
	if sk.Version != "v2" {
		t.Errorf("expected version 'v2', got '%s'", sk.Version)
	}
	if sk.Status != "draft" {
		t.Errorf("expected status 'draft', got '%s'", sk.Status)
	}
}

func TestDeleteSkill(t *testing.T) {
	s := testStore()
	defer s.Close()

	id, _ := s.CreateSkill(newSkill("delete-me", "test"))

	err := s.DeleteSkill(id)
	if err != nil {
		t.Fatalf("delete skill: %v", err)
	}

	sk, err := s.GetSkill(id)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if sk != nil {
		t.Errorf("expected nil after delete, got %+v", sk)
	}
}

// ========== Phase 4: SharedLearningRecord CRUD ==========

func TestCreateAndGetSharedLearning(t *testing.T) {
	s := testStore()
	defer s.Close()

	rec := &SharedLearningRecord{
		SourceType: "defect",
		SourceID:   42,
		TargetURI:  "viking://resources/learnings/test.md",
		Published:  true,
	}
	id, err := s.CreateSharedLearning(rec)
	if err != nil {
		t.Fatalf("create shared learning: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	records, err := s.ListSharedLearning(10)
	if err != nil {
		t.Fatalf("list shared learning: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.SourceType != "defect" {
		t.Errorf("expected source_type 'defect', got '%s'", r.SourceType)
	}
	if r.SourceID != 42 {
		t.Errorf("expected source_id 42, got %d", r.SourceID)
	}
	if r.TargetURI != "viking://resources/learnings/test.md" {
		t.Errorf("expected target_uri, got '%s'", r.TargetURI)
	}
	if !r.Published {
		t.Errorf("expected published=true")
	}
}

func TestListSharedLearning_MultipleTypes(t *testing.T) {
	s := testStore()
	defer s.Close()

	s.CreateSharedLearning(&SharedLearningRecord{SourceType: "defect", SourceID: 1, Published: true})
	s.CreateSharedLearning(&SharedLearningRecord{SourceType: "patch", SourceID: 2, Published: false})
	s.CreateSharedLearning(&SharedLearningRecord{SourceType: "audit", SourceID: 3, Published: true})

	records, err := s.ListSharedLearning(50)
	if err != nil {
		t.Fatalf("list shared learning: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// Check ordering (DESC by id)
	if records[0].SourceType != "audit" {
		t.Errorf("expected newest first (audit), got '%s'", records[0].SourceType)
	}
	if records[2].SourceType != "defect" {
		t.Errorf("expected oldest last (defect), got '%s'", records[2].SourceType)
	}
}

// ========== Phase 4: AuditLog CRUD ==========

func TestCreateAndListAuditLog(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("audit-test", "test content"))

	a := &AuditLog{
		PromptID:     pID,
		RulesChecked: 5,
		RulesPassed:  3,
		RulesFailed:  2,
		Violations:   "missing error handling\\nno input validation",
		Passed:       false,
	}
	id, err := s.CreateAuditLog(a)
	if err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	logs, err := s.ListAuditLogs(10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	l := logs[0]
	if l.PromptID != pID {
		t.Errorf("expected prompt_id %d, got %d", pID, l.PromptID)
	}
	if l.RulesChecked != 5 {
		t.Errorf("expected rules_checked 5, got %d", l.RulesChecked)
	}
	if l.RulesPassed != 3 {
		t.Errorf("expected rules_passed 3, got %d", l.RulesPassed)
	}
	if l.RulesFailed != 2 {
		t.Errorf("expected rules_failed 2, got %d", l.RulesFailed)
	}
	if l.Passed {
		t.Errorf("expected passed=false")
	}
}

func TestListAuditLogs_PassedTrue(t *testing.T) {
	s := testStore()
	defer s.Close()

	pID, _ := s.CreatePrompt(newPrompt("pass-test", "clean content"))

	s.CreateAuditLog(&AuditLog{
		PromptID:     pID,
		RulesChecked: 5,
		RulesPassed:  5,
		RulesFailed:  0,
		Passed:       true,
	})

	logs, err := s.ListAuditLogs(10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if !logs[0].Passed {
		t.Errorf("expected passed=true (bool from DB int)")
	}
}
