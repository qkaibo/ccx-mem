package memory

import (
	"os"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(&StoreConfig{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("创建测试存储失败: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestInsertAndGetMemory(t *testing.T) {
	store := newTestStore(t)

	rec := &MemoryRecord{
		Content: "用户偏好使用 dark mode",
		Layer:   "indexed",
		UserID:  "user-1",
		Tags:    "preference,ui",
		Source:  "manual",
	}

	id, err := store.InsertMemory(rec)
	if err != nil {
		t.Fatalf("插入记忆失败: %v", err)
	}
	if id <= 0 {
		t.Fatalf("期望有效 ID，得到 %d", id)
	}

	got, err := store.GetMemory(id)
	if err != nil {
		t.Fatalf("获取记忆失败: %v", err)
	}
	if got.Content != rec.Content {
		t.Fatalf("内容不匹配: got=%q want=%q", got.Content, rec.Content)
	}
	if got.Layer != rec.Layer {
		t.Fatalf("层级不匹配: got=%q want=%q", got.Layer, rec.Layer)
	}
}

func TestUpdateMemory(t *testing.T) {
	store := newTestStore(t)

	rec := &MemoryRecord{
		Content: "old content",
		Layer:   "indexed",
		UserID:  "user-1",
		Tags:    "test",
		Source:  "manual",
	}
	id, err := store.InsertMemory(rec)
	if err != nil {
		t.Fatalf("插入记忆失败: %v", err)
	}

	rec.ID = id
	rec.Content = "new content"
	rec.Layer = "core"
	rec.Tags = "test,updated"

	if err := store.UpdateMemory(rec); err != nil {
		t.Fatalf("更新记忆失败: %v", err)
	}

	got, err := store.GetMemory(id)
	if err != nil {
		t.Fatalf("获取记忆失败: %v", err)
	}
	if got.Content != "new content" {
		t.Fatalf("内容未更新: got=%q", got.Content)
	}
	if got.Layer != "core" {
		t.Fatalf("层级未更新: got=%q", got.Layer)
	}
}

func TestDeleteMemory(t *testing.T) {
	store := newTestStore(t)

	rec := &MemoryRecord{
		Content: "to be deleted",
		Layer:   "indexed",
		UserID:  "user-1",
	}
	id, _ := store.InsertMemory(rec)

	if err := store.DeleteMemory(id); err != nil {
		t.Fatalf("删除记忆失败: %v", err)
	}

	_, err := store.GetMemory(id)
	if err == nil {
		t.Fatal("期望获取已删除记忆返回错误")
	}
}

func TestListMemoriesByUser(t *testing.T) {
	store := newTestStore(t)

	store.InsertMemory(&MemoryRecord{Content: "global memory", Layer: "indexed", UserID: ""})
	store.InsertMemory(&MemoryRecord{Content: "user-1 memory", Layer: "indexed", UserID: "user-1"})
	store.InsertMemory(&MemoryRecord{Content: "user-2 memory", Layer: "indexed", UserID: "user-2"})

	results, err := store.ListMemoriesByUser("user-1", 10, 0)
	if err != nil {
		t.Fatalf("列出记忆失败: %v", err)
	}

	// user-1 应该看到自己的记忆 + 全局记忆
	if len(results) < 2 {
		t.Fatalf("期望至少 2 条记录，得到 %d", len(results))
	}
}

func TestListMemoryByLayer(t *testing.T) {
	store := newTestStore(t)

	store.InsertMemory(&MemoryRecord{Content: "core-1", Layer: "core", UserID: "user-1"})
	store.InsertMemory(&MemoryRecord{Content: "indexed-1", Layer: "indexed", UserID: "user-1"})

	coreResults, _ := store.ListMemoryByLayer("user-1", "core")
	if len(coreResults) == 0 || coreResults[0].Layer != "core" {
		t.Fatalf("核心记忆列表错误: %+v", coreResults)
	}

	indexedResults, _ := store.ListMemoryByLayer("user-1", "indexed")
	if len(indexedResults) == 0 || indexedResults[0].Layer != "indexed" {
		t.Fatalf("索引记忆列表错误: %+v", indexedResults)
	}
}

func TestSearchMemories(t *testing.T) {
	store := newTestStore(t)

	store.InsertMemory(&MemoryRecord{
		Content: "用户小明喜欢简洁的回复风格",
		Layer:   "indexed",
		UserID:  "user-1",
		Tags:    "preference,style",
	})
	store.InsertMemory(&MemoryRecord{
		Content: "CCX 项目使用 Go 语言开发",
		Layer:   "indexed",
		UserID:  "", // 全局记忆
	})
	store.InsertMemory(&MemoryRecord{
		Content: "用户讨厌猜测性回答，要求先读源码",
		Layer:   "core",
		UserID:  "user-1",
		Tags:    "preference,behavior",
	})

	results, err := store.SearchMemories("user-1", "简洁", 10)
	if err != nil {
		t.Fatalf("全文检索失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("期望至少 1 条匹配结果")
	}
	found := false
	for _, r := range results {
		if r.Content == "用户小明喜欢简洁的回复风格" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("未找到目标记忆: %+v", results)
	}
}

func TestSearchByTag(t *testing.T) {
	store := newTestStore(t)

	store.InsertMemory(&MemoryRecord{
		Content: "核心偏好",
		Layer:   "core",
		UserID:  "user-1",
		Tags:    "preference",
	})

	results, err := store.SearchMemories("user-1", "preference", 10)
	if err != nil {
		t.Fatalf("标签检索失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("期望通过标签 preference 找到记忆")
	}
}

func TestBumpAccessCount(t *testing.T) {
	store := newTestStore(t)

	rec := &MemoryRecord{Content: "test", Layer: "indexed", UserID: "user-1"}
	id, _ := store.InsertMemory(rec)

	store.BumpAccessCount(id)

	got, _ := store.GetMemory(id)
	if got.AccessCount != 1 {
		t.Fatalf("期望 access_count=1，得到 %d", got.AccessCount)
	}
}

func TestInsertTrace(t *testing.T) {
	store := newTestStore(t)

	trace := &ExecutionTrace{
		UserID:         "user-1",
		RequestSummary: "test request",
		Success:        true,
		LatencyMs:      150,
	}

	id, err := store.InsertTrace(trace)
	if err != nil {
		t.Fatalf("插入轨迹失败: %v", err)
	}
	if id <= 0 {
		t.Fatalf("期望有效轨迹 ID")
	}

	traces, err := store.GetRecentTraces(10)
	if err != nil {
		t.Fatalf("获取轨迹失败: %v", err)
	}
	if len(traces) == 0 {
		t.Fatal("期望至少 1 条轨迹")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
