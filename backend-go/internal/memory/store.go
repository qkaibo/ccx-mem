package memory

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// StoreConfig 记忆存储配置
type StoreConfig struct {
	DBPath string // 数据库文件路径，默认 ".config/memory.db"
}

// MemoryRecord 记忆记录
type MemoryRecord struct {
	ID          int64     `json:"id"`
	Content     string    `json:"content"`
	Layer       string    `json:"layer"`   // "core" | "indexed"
	UserID      string    `json:"user_id"` // "" = 全局
	Tags        string    `json:"tags"`    // 逗号分隔
	Source      string    `json:"source"`  // "manual" | "auto-extracted"
	AccessCount int       `json:"access_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ConversationRecord 对话记录
type ConversationRecord struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	MemoryIDs string    `json:"memory_ids"` // 逗号分隔的 memory ids
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

// Store 记忆存储（SQLite 实现）
type Store struct {
	db     *sql.DB
	dbPath string
}

// NewStore 创建记忆存储
func NewStore(cfg *StoreConfig) (*Store, error) {
	if cfg == nil {
		cfg = &StoreConfig{DBPath: ".config/memory.db"}
	}

	// 确保目录存在
	dir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建记忆数据库目录失败: %w", err)
	}

	// 打开 SQLite（WAL + NORMAL 同步）
	dsn := cfg.DBPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开记忆数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db, dbPath: cfg.DBPath}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化记忆 schema 失败: %w", err)
	}

	log.Printf("[Memory] 记忆存储已初始化: %s", cfg.DBPath)
	return store, nil
}

// initSchema 初始化数据库表结构（使用 user_version 做迁移）
func (s *Store) initSchema() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("读取 schema 版本失败: %w", err)
	}

	if version == 0 {
		schema := `
			CREATE TABLE IF NOT EXISTS memories (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				content      TEXT    NOT NULL,
				layer        TEXT    NOT NULL DEFAULT 'indexed',
				user_id      TEXT    NOT NULL DEFAULT '',
				tags         TEXT    NOT NULL DEFAULT '',
				source       TEXT    NOT NULL DEFAULT 'manual',
				access_count INTEGER NOT NULL DEFAULT 0,
				created_at   INTEGER NOT NULL,
				updated_at   INTEGER NOT NULL
			);

			CREATE TABLE IF NOT EXISTS memory_tags (
				id        INTEGER PRIMARY KEY AUTOINCREMENT,
				memory_id INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
				tag       TEXT    NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_memory_tags_tag
				ON memory_tags(tag);

			CREATE INDEX IF NOT EXISTS idx_memory_tags_memory_id
				ON memory_tags(memory_id);

			-- 内容 LIKE 检索索引（替代 FTS5，兼容 CJK）
			CREATE INDEX IF NOT EXISTS idx_memories_content
				ON memories(content);

			CREATE TABLE IF NOT EXISTS conversations (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id    TEXT    NOT NULL DEFAULT '',
				memory_ids TEXT    NOT NULL DEFAULT '',
				summary    TEXT    NOT NULL DEFAULT '',
				created_at INTEGER NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_conversations_user_id
				ON conversations(user_id);

			CREATE TABLE IF NOT EXISTS execution_traces (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id         TEXT    NOT NULL DEFAULT '',
				request_summary TEXT    NOT NULL DEFAULT '',
				success         INTEGER NOT NULL DEFAULT 1,
				error_type      TEXT    NOT NULL DEFAULT '',
				latency_ms      INTEGER NOT NULL DEFAULT 0,
				created_at      INTEGER NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_traces_user_id
				ON execution_traces(user_id);

			CREATE INDEX IF NOT EXISTS idx_traces_created_at
				ON execution_traces(created_at);

			PRAGMA user_version = 1;
		`
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("创建初始 schema 失败: %w", err)
		}
		log.Printf("[Memory-Schema] 初始化 memory schema v1")
	}

	return nil
}

// Close 关闭数据库
func (s *Store) Close() error {
	return s.db.Close()
}

// ==========================================
//  CRUD — 记忆
// ==========================================

// InsertMemory 插入记忆（同时拆写 memory_tags）
func (s *Store) InsertMemory(r *MemoryRecord) (int64, error) {
	now := time.Now().Unix()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	r.UpdatedAt = r.CreatedAt

	result, err := s.db.Exec(`
		INSERT INTO memories (content, layer, user_id, tags, source, access_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, r.Content, r.Layer, r.UserID, r.Tags, r.Source, r.AccessCount, r.CreatedAt.Unix(), now)
	if err != nil {
		return 0, fmt.Errorf("插入记忆失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取记忆 ID 失败: %w", err)
	}

	// 拆写标签
	if r.Tags != "" {
		if err := s.syncTags(id, r.Tags); err != nil {
			log.Printf("[Memory] 警告: 同步标签失败 (memory_id=%d): %v", id, err)
		}
	}

	return id, nil
}

// UpdateMemory 更新记忆
func (s *Store) UpdateMemory(r *MemoryRecord) error {
	now := time.Now().Unix()
	r.UpdatedAt = time.Now()

	res, err := s.db.Exec(`
		UPDATE memories SET content = ?, layer = ?, user_id = ?, tags = ?, source = ?, access_count = ?, updated_at = ?
		WHERE id = ?
	`, r.Content, r.Layer, r.UserID, r.Tags, r.Source, r.AccessCount, now, r.ID)
	if err != nil {
		return fmt.Errorf("更新记忆失败: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("记忆 id=%d 不存在", r.ID)
	}

	// 重新同步标签
	if r.Tags != "" {
		if err := s.syncTags(r.ID, r.Tags); err != nil {
			log.Printf("[Memory] 警告: 同步标签失败 (memory_id=%d): %v", r.ID, err)
		}
	}

	return nil
}

// DeleteMemory 删除记忆
func (s *Store) DeleteMemory(id int64) error {
	res, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除记忆失败: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("记忆 id=%d 不存在", id)
	}
	return nil
}

// GetMemory 获取单条记忆
func (s *Store) GetMemory(id int64) (*MemoryRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, content, layer, user_id, tags, source, access_count, created_at, updated_at
		FROM memories WHERE id = ?
	`, id)

	var r MemoryRecord
	var createdAt, updatedAt int64
	err := row.Scan(&r.ID, &r.Content, &r.Layer, &r.UserID, &r.Tags, &r.Source, &r.AccessCount, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("记忆 id=%d 不存在", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询记忆失败: %w", err)
	}
	r.CreatedAt = time.Unix(createdAt, 0)
	r.UpdatedAt = time.Unix(updatedAt, 0)

	return &r, nil
}

// ListMemoriesByUser 按用户列出记忆
func (s *Store) ListMemoriesByUser(userID string, limit, offset int) ([]MemoryRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, content, layer, user_id, tags, source, access_count, created_at, updated_at
		FROM memories
		WHERE user_id = ? OR user_id = ''
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("列出记忆失败: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// ListMemoryByLayer 按层级列出记忆
func (s *Store) ListMemoryByLayer(userID, layer string) ([]MemoryRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, content, layer, user_id, tags, source, access_count, created_at, updated_at
		FROM memories
		WHERE (user_id = ? OR user_id = '') AND layer = ?
		ORDER BY updated_at DESC
	`, userID, layer)
	if err != nil {
		return nil, fmt.Errorf("按层级列出记忆失败: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// BumpAccessCount 增加记忆访问计数
func (s *Store) BumpAccessCount(id int64) {
	_, err := s.db.Exec(`UPDATE memories SET access_count = access_count + 1 WHERE id = ?`, id)
	if err != nil {
		log.Printf("[Memory] 警告: 更新访问计数失败 (id=%d): %v", id, err)
	}
}

// ==========================================
//  FTS5 全文检索
// ==========================================

// SearchMemories 内容 LIKE 检索 + 标签匹配 + 热度加权
// 返回匹配的记忆，按 access_count DESC 排序（检索频率越高越靠前）。
// 设计决策：使用 LIKE 而非 FTS5，因为 FTS5 不支持 CJK 分词。
// 对于记忆存储这种小数据集（通常 < 10K 条），LIKE 搜索性能足够。
func (s *Store) SearchMemories(userID, query string, maxResults int) ([]MemoryRecord, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	words := strings.Fields(query)
	if len(words) == 0 {
		return nil, nil
	}

	// 只对前 3 个关键词做检索
	wordLimit := 3
	if len(words) < wordLimit {
		wordLimit = len(words)
	}

	// 构建 WHERE 条件
	var conds []string
	for range words[:wordLimit] {
		conds = append(conds, "content LIKE ?")
	}
	whereClause := strings.Join(conds, " AND ")

	// 参数：每个词一个 pattern + user_id
	sqlQuery := fmt.Sprintf(`
		SELECT id, content, layer, user_id, tags, source, access_count, created_at, updated_at
		FROM memories
		WHERE (%s) AND (user_id = ? OR user_id = '')
		ORDER BY access_count DESC
		LIMIT ?
	`, whereClause)

	var sqlArgs []interface{}
	for _, w := range words[:wordLimit] {
		sqlArgs = append(sqlArgs, "%"+w+"%")
	}
	sqlArgs = append(sqlArgs, userID, maxResults)

	rows, err := s.db.Query(sqlQuery, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("检索记忆失败: %w", err)
	}
	defer rows.Close()

	seen := make(map[int64]bool)
	var results []MemoryRecord
	for rows.Next() {
		var m MemoryRecord
		var createdAt, updatedAt int64
		if err := rows.Scan(&m.ID, &m.Content, &m.Layer, &m.UserID, &m.Tags, &m.Source, &m.AccessCount, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描检索结果失败: %w", err)
		}
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		results = append(results, m)
	}

	// 标签检索（补充，去重）
	tagQuery := fmt.Sprintf(`
		SELECT m.id, m.content, m.layer, m.user_id, m.tags, m.source, m.access_count, m.created_at, m.updated_at
		FROM memories m
		JOIN memory_tags mt ON m.id = mt.memory_id
		WHERE (mt.tag LIKE ?) AND (m.user_id = ? OR m.user_id = '')
		ORDER BY m.access_count DESC
		LIMIT ?
	`)

	for _, w := range words[:wordLimit] {
		tagRows, err := s.db.Query(tagQuery, "%"+w+"%", userID, maxResults)
		if err != nil {
			log.Printf("[Memory] 标签检索失败: %v", err)
			continue
		}
		for tagRows.Next() {
			var m MemoryRecord
			var createdAt, updatedAt int64
			if err := tagRows.Scan(&m.ID, &m.Content, &m.Layer, &m.UserID, &m.Tags, &m.Source, &m.AccessCount, &createdAt, &updatedAt); err != nil {
				tagRows.Close()
				return nil, fmt.Errorf("扫描标签检索结果失败: %w", err)
			}
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			m.CreatedAt = time.Unix(createdAt, 0)
			m.UpdatedAt = time.Unix(updatedAt, 0)
			results = append(results, m)
		}
		tagRows.Close()
	}

	return results, nil
}

// ==========================================
//  对话记录
// ==========================================

// InsertConversation 记录对话
func (s *Store) InsertConversation(r *ConversationRecord) (int64, error) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	result, err := s.db.Exec(`
		INSERT INTO conversations (user_id, memory_ids, summary, created_at)
		VALUES (?, ?, ?, ?)
	`, r.UserID, r.MemoryIDs, r.Summary, r.CreatedAt.Unix())
	if err != nil {
		return 0, fmt.Errorf("插入对话记录失败: %w", err)
	}
	return result.LastInsertId()
}

// ==========================================
//  执行轨迹（给 Dreaming 用）
// ==========================================

// ExecutionTrace 执行轨迹
type ExecutionTrace struct {
	ID             int64     `json:"id"`
	UserID         string    `json:"user_id"`
	RequestSummary string    `json:"request_summary"`
	Success        bool      `json:"success"`
	ErrorType      string    `json:"error_type"`
	LatencyMs      int64     `json:"latency_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

// InsertTrace 记录执行轨迹
func (s *Store) InsertTrace(t *ExecutionTrace) (int64, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	success := 0
	if t.Success {
		success = 1
	}
	result, err := s.db.Exec(`
		INSERT INTO execution_traces (user_id, request_summary, success, error_type, latency_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, t.UserID, t.RequestSummary, success, t.ErrorType, t.LatencyMs, t.CreatedAt.Unix())
	if err != nil {
		return 0, fmt.Errorf("插入执行轨迹失败: %w", err)
	}
	return result.LastInsertId()
}

// GetRecentTraces 获取最近的执行轨迹
func (s *Store) GetRecentTraces(limit int) ([]ExecutionTrace, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, request_summary, success, error_type, latency_ms, created_at
		FROM execution_traces
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询执行轨迹失败: %w", err)
	}
	defer rows.Close()

	var traces []ExecutionTrace
	for rows.Next() {
		var t ExecutionTrace
		var success int
		var createdAt int64
		if err := rows.Scan(&t.ID, &t.UserID, &t.RequestSummary, &success, &t.ErrorType, &t.LatencyMs, &createdAt); err != nil {
			return nil, fmt.Errorf("扫描执行轨迹失败: %w", err)
		}
		t.Success = success == 1
		t.CreatedAt = time.Unix(createdAt, 0)
		traces = append(traces, t)
	}
	return traces, nil
}

// ==========================================
//  内部工具函数
// ==========================================

// syncTags 同步 memory_tags 表（删除旧标签 + 插入新标签）
func (s *Store) syncTags(memoryID int64, tags string) error {
	// 删除旧标签
	if _, err := s.db.Exec(`DELETE FROM memory_tags WHERE memory_id = ?`, memoryID); err != nil {
		return err
	}

	// 插入新标签
	for _, tag := range strings.Split(tags, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, err := s.db.Exec(`INSERT INTO memory_tags (memory_id, tag) VALUES (?, ?)`, memoryID, tag); err != nil {
			return err
		}
	}
	return nil
}

// scanMemories 扫描查询结果构建 MemoryRecord 列表
func scanMemories(rows *sql.Rows) ([]MemoryRecord, error) {
	var records []MemoryRecord
	for rows.Next() {
		var r MemoryRecord
		var createdAt, updatedAt int64
		if err := rows.Scan(&r.ID, &r.Content, &r.Layer, &r.UserID, &r.Tags, &r.Source, &r.AccessCount, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描记忆记录失败: %w", err)
		}
		r.CreatedAt = time.Unix(createdAt, 0)
		r.UpdatedAt = time.Unix(updatedAt, 0)
		records = append(records, r)
	}
	return records, nil
}
