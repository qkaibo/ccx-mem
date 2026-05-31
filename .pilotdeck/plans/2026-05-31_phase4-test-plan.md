# ccx-mem Phase 4 测试计划

## 背景

Phase 4 新增 821 行代码，0 个测试。需要补齐测试覆盖。

## 一、store_test.go 扩展 — Skill & SharedLearning CRUD

### 1.1 Skill CRUD
- [ ] `TestCreateAndGetSkill` — 创建 Skill 再 GetSkill 验证所有字段
- [ ] `TestListSkills` — 创建多条 Skill，按 category/status 过滤
- [ ] `TestUpdateSkill` — 更新 Skill content/version/status
- [ ] `TestDeleteSkill` — 删除后再 GetSkill 应返回 nil（无 ErrNoRows）
- [ ] `TestSkillCRUD_EmptyFields` — nil/null 字段的边缘行为

### 1.2 SharedLearningRecord CRUD
- [ ] `TestCreateAndGetSharedLearning` — 创建记录 + 验证
- [ ] `TestListSharedLearning` — 多条记录列表
- [ ] `TestListSharedLearning_FilterByType` — 按 source_type 过滤

### 1.3 AuditLog CRUD
- [ ] `TestCreateAndGetAuditLog` — 创建 + 验证 Passed 字段（DB int → Go bool）
- [ ] `TestListAuditLogs` — 多条列表 + 分页

## 二、publisher_test.go — Publisher 测试

### 2.1 环境检测
- [ ] `TestPublisher_Disabled` — OPENVIKING_ENABLED=false 时所有 publish 方法返回 (nil, nil)

### 2.2 单元测试（需 mock HTTP）
- [ ] 确认 Go http.RoundTripper 可 mock 后：
  - [ ] `TestUploadContent_Success` — HTTP 200 → 返回 URI
  - [ ] `TestUploadContent_HTTPError` — 非 200 → 返回错误
  - [ ] `TestUploadContent_NetworkTimeout` — 超时处理
  - [ ] `TestPublishDefect_RecordCreated` — 发布成功后 SharedLearning 记录写入
  - [ ] `TestPublishDefect_RecordCreatedOnFailure` — 发布失败后仍写入失败记录

### 2.3 集成测试
- [ ] `TestPublishDefect_EndToEnd` — 创建 Defect → PublishDefect → 验证 SharedLearningRecord
- [ ] `TestPublishPatch_EndToEnd` — 同上 for patch
- [ ] `TestPublishAudit_EndToEnd` — 同上 for audit

## 三、api_test.go — API 路由测试

### 3.1 Skill API
- [ ] `TestAPICreateSkill` — POST /api/v2/skills → 201
- [ ] `TestAPIGetSkill` — GET /api/v2/skills/:id → 200 + body
- [ ] `TestAPIListSkills` — GET /api/v2/skills?category=X&status=Y → 200 + array
- [ ] `TestAPIUpdateSkill` — PUT /api/v2/skills/:id → 200
- [ ] `TestAPIDeleteSkill` — DELETE /api/v2/skills/:id → 204
- [ ] `TestAPIGetSkill_NotFound` — 不存在 → 404

### 3.2 Shared Learning API
- [ ] `TestAPIListSharedLearning` — GET /api/v2/shared-learning → 200

### 3.3 Publish API
- [ ] `TestAPIPublishDefect` — POST /api/v2/publish/defect → 200
- [ ] `TestAPIPublishPatch` — POST /api/v2/publish/patch → 200
- [ ] `TestAPIPublishAudit` — POST /api/v2/publish/audit → 200
- [ ] `TestAPISearchOpenViking` — GET /api/v2/search?q=XXX → 200
- [ ] `TestAPIPublish_WhenDisabled` — OPENVIKING_ENABLED=false → 随 publisher 的决定返回

## 四、现有测试验证

- [ ] 确认 `make test` 仍全绿（Phase 4 代码未破坏已有测试）
- [ ] 确认编译通过：`go build ./...`

## 实施方式

- 运行 `make test` 命令需在 Agent 模式下执行
- 推荐按顺序：先扩展 store_test.go，再写 publisher_test.go，最后 api_test.go
- publisher 的 HTTP mock 使用 Go 标准库 `httptest.NewServer` 替代真实的 OpenViking 端点
- 所有测试使用 `:memory:` SQLite 数据库，不依赖外部服务

## 验证标准

- [ ] `make test` 全部通过
- [ ] 新增 Phase 4 相关测试 ≥ 15 个
- [ ] 覆盖率 ≥ 现有水平（Phase 3 覆盖的行不被回归）
