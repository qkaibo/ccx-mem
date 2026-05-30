package evolution

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store manages the evolution SQLite database.
type Store struct {
	db *sql.DB
}

// StoreConfig holds evolution store configuration.
type StoreConfig struct {
	DBPath string
}

// Prompt represents a system prompt or skill template.
type Prompt struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Version   string    `json:"version"`
	Category  string    `json:"category"`
	Hash      string    `json:"hash"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExecutionTrace records a single request execution.
type ExecutionTrace struct {
	ID             int64     `json:"id"`
	PromptID       int64     `json:"prompt_id"`
	UserID         string    `json:"user_id"`
	RequestSummary string    `json:"request_summary"`
	Success        bool      `json:"success"`
	ErrorType      string    `json:"error_type"`
	LatencyMs      int64     `json:"latency_ms"`
	Analyzed       bool      `json:"analyzed"`
	CreatedAt      time.Time `json:"created_at"`
}

// Defect represents an identified issue in a prompt/skill.
type Defect struct {
	ID       int64     `json:"id"`
	PromptID int64     `json:"prompt_id"`
	Type     string    `json:"type"` // discovery, optimization, skill_defect, execution_error
	Evidence string    `json:"evidence"`
	Fixed    bool      `json:"fixed"`
	Severity string    `json:"severity"` // low, medium, high, critical
	CreatedAt time.Time `json:"created_at"`
}

// Skill represents a reusable agent skill (SKILL.md format).
type Skill struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Version     string    `json:"version"`
	Author      string    `json:"author"`
	Tags        string    `json:"tags"`       // comma-separated
	Category    string    `json:"category"`
	Status      string    `json:"status"`     // active, draft, archived
	PromptID    int64     `json:"prompt_id"`  // optional, links to a prompt
	Hash        string    `json:"hash"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SharedLearningRecord records a published learning to OpenViking.
type SharedLearningRecord struct {
	ID            int64     `json:"id"`
	SourceType    string    `json:"source_type"` // defect, patch, audit, skill
	SourceID      int64     `json:"source_id"`
	TargetURI     string    `json:"target_uri"`    // viking://resources/...
	Published     bool      `json:"published"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// AuditLog records a prompt audit result.
type AuditLog struct {
	ID           int64     `json:"id"`
	PromptID     int64     `json:"prompt_id"`
	RulesChecked int       `json:"rules_checked"`
	RulesPassed  int       `json:"rules_passed"`
	RulesFailed  int       `json:"rules_failed"`
	Violations   string    `json:"violations"`
	Passed       bool      `json:"passed"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewStore creates a new evolution store.
func NewStore(cfg *StoreConfig) (*Store, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = ".config/evolution.db"
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open evolution db: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate evolution db: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			content TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT 'v1',
			category TEXT NOT NULL DEFAULT 'system',
			hash TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS execution_traces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt_id INTEGER DEFAULT 0,
			user_id TEXT NOT NULL DEFAULT '',
			request_summary TEXT NOT NULL DEFAULT '',
			success INTEGER NOT NULL DEFAULT 1,
			error_type TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			analyzed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS defects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt_id INTEGER NOT NULL DEFAULT 0,
			type TEXT NOT NULL DEFAULT 'discovery',
			evidence TEXT NOT NULL DEFAULT '',
			fixed INTEGER NOT NULL DEFAULT 0,
			severity TEXT NOT NULL DEFAULT 'medium',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt_id INTEGER NOT NULL DEFAULT 0,
			rules_checked INTEGER NOT NULL DEFAULT 0,
			rules_passed INTEGER NOT NULL DEFAULT 0,
			rules_failed INTEGER NOT NULL DEFAULT 0,
			violations TEXT NOT NULL DEFAULT '',
			passed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT 'v1',
			author TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT 'general',
			status TEXT NOT NULL DEFAULT 'draft',
			prompt_id INTEGER NOT NULL DEFAULT 0,
			hash TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS shared_learning (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_type TEXT NOT NULL DEFAULT '',
			source_id INTEGER NOT NULL DEFAULT 0,
			target_uri TEXT NOT NULL DEFAULT '',
			published INTEGER NOT NULL DEFAULT 0,
			published_at TEXT,
			error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w (query: %.60s...)", err, q)
		}
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Prompt CRUD ---

func (s *Store) CreatePrompt(p *Prompt) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO prompts (name, content, version, category, hash, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.Name, p.Content, p.Version, p.Category, p.Hash, p.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("create prompt: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) GetPrompt(id int64) (*Prompt, error) {
	row := s.db.QueryRow(`SELECT id, name, content, version, category, hash, status, created_at, updated_at FROM prompts WHERE id = ?`, id)
	p := &Prompt{}
	var createdAt, updatedAt string
	err := row.Scan(&p.ID, &p.Name, &p.Content, &p.Version, &p.Category, &p.Hash, &p.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", updatedAt)
	return p, nil
}

func (s *Store) ListPrompts(limit, offset int) ([]*Prompt, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, name, content, version, category, hash, status, created_at, updated_at FROM prompts ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	defer rows.Close()

	var prompts []*Prompt
	for rows.Next() {
		p := &Prompt{}
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Content, &p.Version, &p.Category, &p.Hash, &p.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt: %w", err)
		}
		p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
		p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", updatedAt)
		prompts = append(prompts, p)
	}
	return prompts, nil
}

func (s *Store) UpdatePrompt(id int64, p *Prompt) error {
	_, err := s.db.Exec(
		`UPDATE prompts SET name=?, content=?, version=?, category=?, hash=?, status=?, updated_at=datetime('now') WHERE id=?`,
		p.Name, p.Content, p.Version, p.Category, p.Hash, p.Status, id,
	)
	return err
}

func (s *Store) DeletePrompt(id int64) error {
	_, err := s.db.Exec(`DELETE FROM prompts WHERE id=?`, id)
	return err
}

// --- Trace CRUD ---

func (s *Store) RecordTrace(t *ExecutionTrace) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO execution_traces (prompt_id, user_id, request_summary, success, error_type, latency_ms)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.PromptID, t.UserID, t.RequestSummary, boolToInt(t.Success), t.ErrorType, t.LatencyMs,
	)
	if err != nil {
		return 0, fmt.Errorf("record trace: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) GetRecentTraces(limit int) ([]*ExecutionTrace, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, prompt_id, user_id, request_summary, success, error_type, latency_ms, analyzed, created_at
		 FROM execution_traces ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent traces: %w", err)
	}
	defer rows.Close()
	return scanTraces(rows)
}

func (s *Store) GetUnanalyzedTraces(limit int) ([]*ExecutionTrace, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, prompt_id, user_id, request_summary, success, error_type, latency_ms, analyzed, created_at
		 FROM execution_traces WHERE analyzed = 0 ORDER BY id ASC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get unanalyzed traces: %w", err)
	}
	defer rows.Close()
	return scanTraces(rows)
}

func (s *Store) MarkTracesAnalyzed(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	// Build IN clause
	query := `UPDATE execution_traces SET analyzed = 1 WHERE id IN (`
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"
	_, err := s.db.Exec(query, args...)
	return err
}

// --- Defect CRUD ---

func (s *Store) CreateDefect(d *Defect) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO defects (prompt_id, type, evidence, severity) VALUES (?, ?, ?, ?)`,
		d.PromptID, d.Type, d.Evidence, d.Severity,
	)
	if err != nil {
		return 0, fmt.Errorf("create defect: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) ListDefects(limit int) ([]*Defect, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, prompt_id, type, evidence, fixed, severity, created_at FROM defects ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list defects: %w", err)
	}
	defer rows.Close()

	var defects []*Defect
	for rows.Next() {
		d := &Defect{}
		if err := rows.Scan(&d.ID, &d.PromptID, &d.Type, &d.Evidence, &d.Fixed, &d.Severity, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan defect: %w", err)
		}
		defects = append(defects, d)
	}
	return defects, nil
}

func (s *Store) MarkDefectFixed(id int64) error {
	_, err := s.db.Exec(`UPDATE defects SET fixed = 1 WHERE id = ?`, id)
	return err
}

func (s *Store) GetDefect(id int64) (*Defect, error) {
	d := &Defect{}
	var createdAt string
	err := s.db.QueryRow(
		`SELECT id, prompt_id, type, evidence, fixed, severity, created_at FROM defects WHERE id = ?`, id,
	).Scan(&d.ID, &d.PromptID, &d.Type, &d.Evidence, &d.Fixed, &d.Severity, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get defect: %w", err)
	}
	d.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
	return d, nil
}

func (s *Store) GetDefectsByPromptID(promptID int64) ([]*Defect, error) {
	rows, err := s.db.Query(
		`SELECT id, prompt_id, type, evidence, fixed, severity, created_at FROM defects WHERE prompt_id = ? ORDER BY id DESC`, promptID,
	)
	if err != nil {
		return nil, fmt.Errorf("get defects by prompt: %w", err)
	}
	defer rows.Close()

	var defects []*Defect
	for rows.Next() {
		d := &Defect{}
		var createdAt string
		if err := rows.Scan(&d.ID, &d.PromptID, &d.Type, &d.Evidence, &d.Fixed, &d.Severity, &createdAt); err != nil {
			return nil, fmt.Errorf("scan defect: %w", err)
		}
		d.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
		defects = append(defects, d)
	}
	return defects, nil
}

// --- Audit Log ---

func (s *Store) CreateAuditLog(a *AuditLog) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO audit_logs (prompt_id, rules_checked, rules_passed, rules_failed, violations, passed) VALUES (?, ?, ?, ?, ?, ?)`,
		a.PromptID, a.RulesChecked, a.RulesPassed, a.RulesFailed, a.Violations, boolToInt(a.Passed),
	)
	if err != nil {
		return 0, fmt.Errorf("create audit log: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) ListAuditLogs(limit int) ([]*AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, prompt_id, rules_checked, rules_passed, rules_failed, violations, passed, created_at FROM audit_logs ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		var passed int
		var createdAt string
		if err := rows.Scan(&l.ID, &l.PromptID, &l.RulesChecked, &l.RulesPassed, &l.RulesFailed, &l.Violations, &passed, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		l.Passed = passed != 0
		l.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
		logs = append(logs, l)
	}
	return logs, nil
}

// --- Skill CRUD ---

func (s *Store) CreateSkill(sk *Skill) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO skills (name, description, content, version, author, tags, category, status, prompt_id, hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sk.Name, sk.Description, sk.Content, sk.Version, sk.Author, sk.Tags, sk.Category, sk.Status, sk.PromptID, sk.Hash,
	)
	if err != nil {
		return 0, fmt.Errorf("create skill: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) GetSkill(id int64) (*Skill, error) {
	sk := &Skill{}
	var createdAt, updatedAt string
	var promptID int64
	err := s.db.QueryRow(
		`SELECT id, name, description, content, version, author, tags, category, status, prompt_id, hash, created_at, updated_at FROM skills WHERE id = ?`, id,
	).Scan(&sk.ID, &sk.Name, &sk.Description, &sk.Content, &sk.Version, &sk.Author, &sk.Tags, &sk.Category, &sk.Status, &promptID, &sk.Hash, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get skill: %w", err)
	}
	sk.PromptID = promptID
	sk.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
	sk.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", updatedAt)
	return sk, nil
}

func (s *Store) ListSkills(category, status string, limit, offset int) ([]*Skill, error) {
	if limit <= 0 {
		limit = 50
	}
	var query string
	var args []interface{}
	if category != "" && status != "" {
		query = `SELECT id, name, description, content, version, author, tags, category, status, prompt_id, hash, created_at, updated_at FROM skills WHERE category=? AND status=? ORDER BY id DESC LIMIT ? OFFSET ?`
		args = append(args, category, status, limit, offset)
	} else if category != "" {
		query = `SELECT id, name, description, content, version, author, tags, category, status, prompt_id, hash, created_at, updated_at FROM skills WHERE category=? ORDER BY id DESC LIMIT ? OFFSET ?`
		args = append(args, category, limit, offset)
	} else if status != "" {
		query = `SELECT id, name, description, content, version, author, tags, category, status, prompt_id, hash, created_at, updated_at FROM skills WHERE status=? ORDER BY id DESC LIMIT ? OFFSET ?`
		args = append(args, status, limit, offset)
	} else {
		query = `SELECT id, name, description, content, version, author, tags, category, status, prompt_id, hash, created_at, updated_at FROM skills ORDER BY id DESC LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var skills []*Skill
	for rows.Next() {
		sk := &Skill{}
		var createdAt, updatedAt string
		var promptID int64
		if err := rows.Scan(&sk.ID, &sk.Name, &sk.Description, &sk.Content, &sk.Version, &sk.Author, &sk.Tags, &sk.Category, &sk.Status, &promptID, &sk.Hash, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		sk.PromptID = promptID
		sk.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
		sk.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", updatedAt)
		skills = append(skills, sk)
	}
	return skills, nil
}

func (s *Store) UpdateSkill(id int64, sk *Skill) error {
	_, err := s.db.Exec(
		`UPDATE skills SET name=?, description=?, content=?, version=?, author=?, tags=?, category=?, status=?, prompt_id=?, hash=?, updated_at=datetime('now') WHERE id=?`,
		sk.Name, sk.Description, sk.Content, sk.Version, sk.Author, sk.Tags, sk.Category, sk.Status, sk.PromptID, sk.Hash, id,
	)
	return err
}

func (s *Store) DeleteSkill(id int64) error {
	_, err := s.db.Exec(`DELETE FROM skills WHERE id=?`, id)
	return err
}

// --- Shared Learning Record CRUD ---

func (s *Store) CreateSharedLearning(r *SharedLearningRecord) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO shared_learning (source_type, source_id, target_uri, published, error_message)
		 VALUES (?, ?, ?, ?, ?)`,
		r.SourceType, r.SourceID, r.TargetURI, boolToInt(r.Published), r.ErrorMessage,
	)
	if err != nil {
		return 0, fmt.Errorf("create shared learning record: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) ListSharedLearning(limit int) ([]*SharedLearningRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, source_type, source_id, target_uri, published, published_at, error_message, created_at
		 FROM shared_learning ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list shared learning: %w", err)
	}
	defer rows.Close()

	var records []*SharedLearningRecord
	for rows.Next() {
		r := &SharedLearningRecord{}
		var published int
		var publishedAt *string
		var createdAt string
		if err := rows.Scan(&r.ID, &r.SourceType, &r.SourceID, &r.TargetURI, &published, &publishedAt, &r.ErrorMessage, &createdAt); err != nil {
			return nil, fmt.Errorf("scan shared learning: %w", err)
		}
		r.Published = published != 0
		if publishedAt != nil {
			t, err := time.Parse("2006-01-02T15:04:05Z07:00", *publishedAt)
			if err == nil {
				r.PublishedAt = &t
			}
		}
		r.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
		records = append(records, r)
	}
	return records, nil
}

// --- Helpers ---

func scanTraces(rows *sql.Rows) ([]*ExecutionTrace, error) {
	var traces []*ExecutionTrace
	for rows.Next() {
		t := &ExecutionTrace{}
		var analyzed int
		var createdAt string
		if err := rows.Scan(&t.ID, &t.PromptID, &t.UserID, &t.RequestSummary, &t.Success, &t.ErrorType, &t.LatencyMs, &analyzed, &createdAt); err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}
		t.Analyzed = analyzed != 0
		t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z07:00", createdAt)
		traces = append(traces, t)
	}
	return traces, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
