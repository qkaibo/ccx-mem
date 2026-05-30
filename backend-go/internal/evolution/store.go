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
		if err := rows.Scan(&l.ID, &l.PromptID, &l.RulesChecked, &l.RulesPassed, &l.RulesFailed, &l.Violations, &passed, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		l.Passed = passed != 0
		logs = append(logs, l)
	}
	return logs, nil
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
