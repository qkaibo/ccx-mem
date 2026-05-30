package memory

import (
	"encoding/json"
	"testing"
)

func TestInjectBasic(t *testing.T) {
	store := newTestStore(t)

	// 插入核心记忆
	store.InsertMemory(&MemoryRecord{
		Content: "用户偏好简洁回复风格",
		Layer:   "core",
		UserID:  "user-1",
	})

	// 插入索引记忆
	store.InsertMemory(&MemoryRecord{
		Content: "用户使用 Go 语言开发",
		Layer:   "indexed",
		UserID:  "user-1",
		Tags:    "language,preference",
	})

	injector := NewInjector(store, DefaultInjectorConfig())

	// 构造一个简单的 chat 请求
	body := map[string]interface{}{
		"model": "deepseek-v4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "帮我写一个 Go 函数",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	modified, changed := injector.Inject(bodyBytes, "user-1", false)
	if !changed {
		t.Fatal("期望注入成功")
	}

	// 验证注入结果
	var result map[string]interface{}
	if err := json.Unmarshal(modified, &result); err != nil {
		t.Fatalf("解析修改后请求体失败: %v", err)
	}

	messages := result["messages"].([]interface{})
	if len(messages) < 2 {
		t.Fatalf("期望至少 2 条消息，得到 %d", len(messages))
	}

	firstMsg := messages[0].(map[string]interface{})
	if firstMsg["role"] != "system" {
		t.Fatalf("期望第一条消息是 system，得到 %v", firstMsg["role"])
	}

	content := firstMsg["content"].(string)
	if content == "" {
		t.Fatal("system 消息内容为空")
	}
	t.Logf("注入后的 system content:\n%s", content)
}

func TestInjectWithExistingSystem(t *testing.T) {
	store := newTestStore(t)

	store.InsertMemory(&MemoryRecord{
		Content: "用户偏好暗色主题",
		Layer:   "core",
		UserID:  "user-1",
	})

	injector := NewInjector(store, DefaultInjectorConfig())

	// 请求已有 system 消息
	body := map[string]interface{}{
		"model": "deepseek-v4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "system",
				"content": "你是一个有用的助手",
			},
			map[string]interface{}{
				"role":    "user",
				"content": "你好",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	modified, changed := injector.Inject(bodyBytes, "user-1", false)
	if !changed {
		t.Fatal("期望注入成功")
	}

	var result map[string]interface{}
	json.Unmarshal(modified, &result)

	messages := result["messages"].([]interface{})
	if len(messages) != 2 {
		t.Fatalf("期望消息数量不变（2条），得到 %d", len(messages))
	}

	firstMsg := messages[0].(map[string]interface{})
	if firstMsg["role"] != "system" {
		t.Fatal("期望第一条仍是 system")
	}

	content := firstMsg["content"].(string)
	// 应包含原始 system 内容 + 注入的记忆
	if content == "你是一个有用的助手" {
		t.Fatal("system 内容应包含注入的记忆，但没有")
	}
	t.Logf("追加后的 system content:\n%s", content)
}

func TestInjectNoMemories(t *testing.T) {
	store := newTestStore(t)
	injector := NewInjector(store, DefaultInjectorConfig())

	body := map[string]interface{}{
		"model": "deepseek-v4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "hello",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	modified, changed := injector.Inject(bodyBytes, "user-1", false)
	if changed {
		t.Fatal("没有记忆时不应该修改请求")
	}

	if string(modified) != string(bodyBytes) {
		t.Fatal("没有记忆时请求体不应改变")
	}
}

func TestInjectDisabled(t *testing.T) {
	store := newTestStore(t)

	store.InsertMemory(&MemoryRecord{
		Content: "test",
		Layer:   "core",
		UserID:  "user-1",
	})

	cfg := DefaultInjectorConfig()
	cfg.Enabled = false
	injector := NewInjector(store, cfg)

	body := map[string]interface{}{
		"model": "deepseek-v4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "hello",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	modified, changed := injector.Inject(bodyBytes, "user-1", false)
	if changed {
		t.Fatal("注入禁用时不应修改")
	}
	if string(modified) != string(bodyBytes) {
		t.Fatal("注入禁用时请求体不应改变")
	}
}

func TestExtractContext(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "ignore"},
			map[string]interface{}{"role": "user", "content": "帮我写代码"},
			map[string]interface{}{"role": "assistant", "content": "好的，什么语言？"},
			map[string]interface{}{"role": "user", "content": "Go 语言"},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	ctx := extractContext(bodyBytes, 3)
	if ctx == "" {
		t.Fatal("期望提取到上下文")
	}
	t.Logf("提取的上下文: %q", ctx)
}

func TestExtractContextArrayContent(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "你好"},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	ctx := extractContext(bodyBytes, 3)
	if ctx == "" {
		t.Fatal("期望从数组 content 中提取到文本")
	}
	t.Logf("提取的上下文: %q", ctx)
}
