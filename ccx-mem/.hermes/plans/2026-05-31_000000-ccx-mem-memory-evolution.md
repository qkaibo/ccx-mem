# ccx-mem：记忆子系统 + 自进化子系统 实现计划

> 生成于 2026-05-31。Phase 0 已完成，Phase 1-2 待实施。

---

## 目标

在 CCX（LLM 代理网关，Go + Gin + SQLite）上添加两个子系统：

1. **记忆子系统（Memory）**：会话间持久记忆，在请求中注入历史记忆
2. **自进化子系统（Self-Evolution）**：分析执行轨迹 → 发现缺陷 → 自动补丁

全部 Go，零外部服务依赖。

---

## 工程现状

| 项目 | 状态 |
|------|------|
| 仓库 | `qkaibo/ccx-mem`（CCX fork，含完整 Git 历史） |
| 本地路径 | `/home/ts/ccx-mem/ccx-mem/` |
| Go 版本 | 1.24.7 |
| 编译 | `make build` 通过，二进制 `./bin/ccx-mem` |
| 上游 | `BenedictKing/CCX` commit `2931491` |

---

## Phase 0：工程搭建 ✅ 已完成

- [x] 克隆 CCX 源码到本地
- [x] 推送到 qkaibo/ccx-mem
- [x] 编译验证通过
- [x] Makefile 就位（build/run/test/clean/docker/lint）

---

## Phase 1：记忆子系统（Memory）

### 1.1 数据模型

根据对 Mem0、Letta Code、Zep 三大主流方案的研究，采用 **分层记忆** 架构。

```sql
-- 记忆主表
CREATE TABLE memories (
    id           TEXT PRIMARY KEY,
    content      TEXT NOT NULL,
    layer        TEXT NOT NULL DEFAULT 'indexed',  -- 'core' | 'indexed'
    type         TEXT NOT NULL,                    -- 'preference' | 'fact' | 'lesson' | 'procedure'
    user_id      TEXT NOT NULL,
    tags         TEXT DEFAULT '',                  -- 逗号分隔
    source       TEXT DEFAULT 'manual',            -- 'manual' | 'auto-extracted'
    access_count INTEGER DEFAULT 0,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- FTS5 全文索引
CREATE VIRTUAL TABLE memory_fts USING fts5(content, content=memories, content_rowid=rowid);

-- 执行轨迹（供 dreaming 用）
CREATE TABLE execution_traces (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    request_summary TEXT,
    success         INTEGER DEFAULT 1,
    error_type      TEXT,
    latency_ms      INTEGER,
    created_at      TEXT NOT NULL
);
```

**设计来源**：
- Mem0 的 ADD-only 记忆策略、实体链接（简化版用 FTS5 替代）
- Letta Code 的 `system/` 层（core 记忆永远注入）vs `projects/` 层（按需检索）
- Zep 的知识图谱思路（简化版用标签代替）

### 1.2 模块分工

| 文件 | 职责 |
|------|------|
| `backend-go/internal/memory/store.go` | SQLite 建表 + CRUD（modernc.org/sqlite） |
| `backend-go/internal/memory/retrieval.go` | 双策略检索：FTS5 全文 + 标签匹配 |
| `backend-go/internal/memory/injector.go` | 请求注入：提取 user_id → 检索 → 拼 system message |
| `backend-go/internal/memory/api.go` | REST API（Gin 路由组） |
| `backend-go/internal/memory/dreaming.go` | 后台分析轨迹，自动提取记忆 |

### 1.3 注入流程

```
chat/handler.go 第 82-91 行之间（SanitizeMalformedThinkingBlocks 之后，LogOriginalRequest 之前）

1. 提取 user_id（从请求 body 的 messages[0].content 或 header）
2. InjectCoreMemory(user_id)
   → 查询 layer='core' 的记忆
   → 拼成一条 system message，插入 messages 数组最前面
3. RetrieveIndexedMemories(user_id, 最近 3 条 user message)
   → FTS5 搜索 + 标签匹配
   → 按 `相关性 × log(1 + access_count)` 排序
   → 取前 N 条（可配置，默认 5）
   → 追加到 system message 后面
4. 更新 access_count
```

### 1.4 REST API

```
GET    /api/memories?user_id=xxx&q=search   → 检索
POST   /api/memories                        → 新增
PUT    /api/memories/:id                     → 修改
DELETE /api/memories/:id                     → 删除
POST   /api/memories/_batch                  → 批量导入
GET    /api/memories/_traces?user_id=xxx     → 查询轨迹
POST   /api/memories/_dream                  → 手动触发 dreaming
```

### 1.5 配置

```json
{
  "memory": {
    "enabled": true,
    "max_indexed_per_request": 5,
    "strategy": "fts5",
    "db_path": ".config/memory.db",
    "dreaming_interval": "30m",
    "dreaming_max_traces": 100
  }
}
```

### 1.6 文件改动清单

| 文件 | 操作 |
|------|------|
| `backend-go/internal/memory/store.go` | 新建 |
| `backend-go/internal/memory/retrieval.go` | 新建 |
| `backend-go/internal/memory/injector.go` | 新建 |
| `backend-go/internal/memory/api.go` | 新建 |
| `backend-go/internal/memory/dreaming.go` | 新建 |
| `backend-go/internal/handlers/chat/handler.go` | 修改：注入点插入 InjectMemory 调用 |
| `backend-go/internal/config/config.go` | 修改：增加 MemoryConfig 结构体 |
| `backend-go/main.go` | 修改：注册 /api/memories 路由、启动 dreaming goroutine |
| `.config/config.json` | 修改：增加 memory 配置节 |

---

## Phase 2：自进化子系统（Self-Evolution）

### 2.1 设计理念

参考 Hermes Agent 的自进化体系，但简化：
- Hermes 用 Python + SKILL.md + 文件系统
- ccx-mem 用 Go + SQLite + structured patching

核心差异：**不调 LLM**。所有分析靠 heuristic，所有补丁靠 pattern matching。

### 2.2 数据模型

```sql
CREATE TABLE prompts (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    content    TEXT NOT NULL,
    version    INTEGER DEFAULT 1,
    hash       TEXT,
    status     TEXT DEFAULT 'active',  -- 'active' | 'deprecated' | 'testing'
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE skill_audits (
    id              TEXT PRIMARY KEY,
    prompt_id       TEXT NOT NULL,
    rules_violated  TEXT,  -- JSON array of rule names
    passed          INTEGER,
    details         TEXT,  -- audit detail text
    created_at      TEXT NOT NULL
);
```

### 2.3 模块分工

| 文件 | 职责 |
|------|------|
| `backend-go/internal/evolution/store.go` | SQLite 建表 + CRUD |
| `backend-go/internal/evolution/analyzer.go` | 分析 execution_traces 发现缺陷 |
| `backend-go/internal/evolution/auditor.go` | 9 条审计规则（SkillEvolver 移植） |
| `backend-go/internal/evolution/patcher.go` | 补丁式更新 prompt |
| `backend-go/internal/evolution/api.go` | REST API |
| `backend-go/internal/evolution/loop.go` | 后台 goroutine 驱动进化循环 |

### 2.4 进化循环

```
每 N 分钟（可配置）:
1. 查询最近 100 条 execution_traces
2. analyzer.Analyze() → 分类为：
   - discovery  （新场景，无对应 prompt）
   - defect     （prompt 规则错误）
   - mistake    （规则对，没执行对）
3. 对 discovery/defect：生成 prompt 补丁
4. auditor.Audit(patch) → 9 条规则检查
5. 通过审计 → patcher.Apply(patch)
6. 不通过 → 记录 audit_log
```

### 2.5 9 条审计规则（从 SkillEvolver 移植）

1. **框架借用** — prompt 不允许引用外部框架特定概念
2. **硬编码字面量** — 不允许 magic number/string 未参数化
3. **脚本膨胀** — 单 prompt 不超过 2000 字符
4. **不可追踪断言** — 不允许 "always"/"never" 无条件判断
5. **形状烘焙** — 输入输出格式必须独立声明
6. **交叉引用** — prompt 间引用必须显式标注版本号
7. **欠抽象** — 3 个以上重复模式必须提取
8. **主操作提升** — 核心操作必须在 prompt 前 3 行出现
9. **静默绕过** — 不允许 "ignore" / "skip" type 指令

### 2.6 REST API

```
GET    /api/evolution/prompts               → 列表
POST   /api/evolution/prompts               → 创建
PUT    /api/evolution/prompts/:id            → 修改
DELETE /api/evolution/prompts/:id            → 删除
POST   /api/evolution/_analyze               → 手动触发分析
POST   /api/evolution/_audit/:prompt_id       → 手动审计
GET    /api/evolution/_traces?prompt_id=xxx  → 查询进化轨迹
```

### 2.7 配置

```json
{
  "evolution": {
    "enabled": true,
    "interval": "1h",
    "max_traces_per_analysis": 100,
    "auto_apply": false,
    "require_audit": true,
    "db_path": ".config/evolution.db"
  }
}
```

### 2.8 文件改动清单

| 文件 | 操作 |
|------|------|
| `backend-go/internal/evolution/store.go` | 新建 |
| `backend-go/internal/evolution/analyzer.go` | 新建 |
| `backend-go/internal/evolution/auditor.go` | 新建 |
| `backend-go/internal/evolution/patcher.go` | 新建 |
| `backend-go/internal/evolution/api.go` | 新建 |
| `backend-go/internal/evolution/loop.go` | 新建 |
| `backend-go/internal/config/config.go` | 修改：增加 EvolutionConfig 结构体 |
| `backend-go/main.go` | 修改：注册 /api/evolution 路由、启动 loop goroutine |

---

## Phase 3：集成与交付

### 3.1 集成点

| 交叉点 | 说明 |
|--------|------|
| Memory ↔ Evolution | execution_traces 表由 memory 写入，evolution 读取 |
| Memory ↔ Chat Handler | injector.go 在 handler.go 注入点调用 |
| Config ↔ 两者 | 统一的 JSON 配置加载，两个子系统各读各的节 |

### 3.2 交付物

- [ ] 编译通过
- [ ] 单元测试（SQLite 内存数据库）
- [ ] 更新 Makefile
- [ ] 更新 .gitignore（排除 .config/*.db）
- [ ] 推送到 qkaibo/ccx-mem

---

## 设计来源（完整链）

| 设计点 | 来源 |
|--------|------|
| 分层记忆（core/indexed） | Letta Code MemFS + Hermes Agent memory tool |
| ADD-only 提取策略 | Mem0 2026.4 新算法 |
| FTS5 全文检索（非向量） | 嵌入式场景最优，CCX 已依赖 modernc.org/sqlite |
| 不使用外部 LLM | CCX 是网关，不是 LLM consumer |
| 轨迹驱动的 dreaming | Letta Dreaming + Hermes 自进化 |
| 9 条审计规则 | SkillEvolver（清华） |
| 补丁式更新 | Hermes agent-skill-evolution-v2 |
| 注入点位置 | 直接阅读 CCX chat/handler.go 源码 |

---

## 实施顺序

1. **Phase 1.1** — `memory/store.go`（SQLite 建表 + CRUD）
2. **Phase 1.2** — `memory/retrieval.go`（FTS5 检索）
3. **Phase 1.3** — `memory/injector.go` + `handler.go` 注入点
4. **Phase 1.4** — `memory/api.go`（REST endpoints）
5. **Phase 1.5** — `memory/dreaming.go`（后台轨迹分析）
6. **Phase 1.6** — 配置 + main.go 集成
7. **Phase 2.1** — `evolution/store.go`
8. **Phase 2.2** — `evolution/tracker.go`（复用 memory 的 execution_traces）
9. **Phase 2.3** — `evolution/analyzer.go`
10. **Phase 2.4** — `evolution/auditor.go` + `patcher.go`
11. **Phase 2.5** — `evolution/loop.go` + main.go 集成
12. **Phase 3** — 测试、Makefile、推送
