package memory

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// InjectorConfig 注入器配置
type InjectorConfig struct {
	Enabled            bool // 是否启用记忆注入
	MaxCoreMemories    int  // 核心记忆最大数量（默认 3）
	MaxIndexedMemories int  // 索引记忆最大数量（默认 5）
}

// DefaultInjectorConfig 默认注入器配置
func DefaultInjectorConfig() InjectorConfig {
	return InjectorConfig{
		Enabled:            true,
		MaxCoreMemories:    3,
		MaxIndexedMemories: 5,
	}
}

// Injector 记忆注入器
type Injector struct {
	store  *Store
	config InjectorConfig
}

// NewInjector 创建记忆注入器
func NewInjector(store *Store, config InjectorConfig) *Injector {
	return &Injector{store: store, config: config}
}

// Inject 向请求体注入记忆（返回修改后的 bodyBytes 和是否修改的标志）
// 在 chat handler 预处理完成后调用（SanitizeMalformedThinkingBlocks 之后，LogOriginalRequest 之前）
func (inj *Injector) Inject(bodyBytes []byte, userID string, enableLogs bool) ([]byte, bool) {
	if !inj.config.Enabled {
		return bodyBytes, false
	}

	// 1. 提取核心记忆（永远注入）
	coreMemories, err := inj.store.ListMemoryByLayer(userID, "core")
	if err != nil {
		if enableLogs {
			log.Printf("[Memory-Inject] 获取核心记忆失败: %v", err)
		}
		coreMemories = nil
	}

	// 2. 从最近消息提取上下文，检索索引记忆
	context := extractContext(bodyBytes, 3)
	var indexedMemories []MemoryRecord
	if context != "" {
		indexedMemories, err = inj.store.SearchMemories(userID, context, inj.config.MaxIndexedMemories)
		if err != nil {
			if enableLogs {
				log.Printf("[Memory-Inject] 检索索引记忆失败: %v", err)
			}
			indexedMemories = nil
		}
	}

	// 如果两样都没有，不修改
	if len(coreMemories) == 0 && len(indexedMemories) == 0 {
		return bodyBytes, false
	}

	// 3. 构建记忆段落
	memoryBlock := buildMemoryBlock(coreMemories, indexedMemories, inj.config.MaxCoreMemories)
	if memoryBlock == "" {
		return bodyBytes, false
	}

	// 4. 注入到 messages 数组（前面插入一条 system message）
	modified, err := injectSystemMessage(bodyBytes, memoryBlock)
	if err != nil {
		if enableLogs {
			log.Printf("[Memory-Inject] 注入记忆失败: %v", err)
		}
		return bodyBytes, false
	}

	if enableLogs {
		log.Printf("[Memory-Inject] 已注入 %d 条核心记忆 + %d 条索引记忆 (user=%s)",
			min(len(coreMemories), inj.config.MaxCoreMemories),
			len(indexedMemories), userID)
	}

	// 更新访问计数
	for i := range indexedMemories {
		inj.store.BumpAccessCount(indexedMemories[i].ID)
	}

	return modified, true
}

// extractContext 从请求体的最近 N 条消息中提取搜索上下文
// 取最后 N 条 user/assistant 消息的文本内容，用空格连接
func extractContext(bodyBytes []byte, maxMessages int) string {
	messages := gjson.GetBytes(bodyBytes, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return ""
	}

	arr := messages.Array()
	if len(arr) == 0 {
		return ""
	}

	var texts []string
	count := 0

	// 从后往前取
	for i := len(arr) - 1; i >= 0 && count < maxMessages; i-- {
		msg := arr[i]
		role := msg.Get("role").String()
		if role != "user" && role != "assistant" {
			continue
		}

		content := msg.Get("content")
		if content.Type == gjson.String {
			t := strings.TrimSpace(content.String())
			if t != "" {
				texts = append(texts, t)
				count++
			}
		} else if content.IsArray() {
			// content 是数组格式：提取 text 类型的内容
			for _, block := range content.Array() {
				if block.Get("type").String() == "text" {
					t := strings.TrimSpace(block.Get("text").String())
					if t != "" {
						texts = append(texts, t)
						break
					}
				}
			}
			if len(texts) > count { // 有新增
				count++
			}
		}
	}

	// 反转回正序（从旧到新）
	for i, j := 0, len(texts)-1; i < j; i, j = i+1, j-1 {
		texts[i], texts[j] = texts[j], texts[i]
	}

	return strings.Join(texts, " ")
}

// buildMemoryBlock 构建记忆文本块
func buildMemoryBlock(coreMemories, indexedMemories []MemoryRecord, maxCore int) string {
	var parts []string

	// 核心记忆
	if len(coreMemories) > 0 {
		limit := maxCore
		if limit > len(coreMemories) {
			limit = len(coreMemories)
		}
		coreParts := make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			coreParts = append(coreParts, "- "+coreMemories[i].Content)
		}
		parts = append(parts, "[Memory]\n"+strings.Join(coreParts, "\n"))
	}

	// 索引记忆
	if len(indexedMemories) > 0 {
		idxParts := make([]string, 0, len(indexedMemories))
		for _, m := range indexedMemories {
			idxParts = append(idxParts, "- "+m.Content)
		}
		parts = append(parts, "[Context]\n"+strings.Join(idxParts, "\n"))
	}

	return strings.Join(parts, "\n\n")
}

// injectSystemMessage 向 messages 数组最前面插入一条 system 消息
// 如果已有 system 消息，则追加到其 content 末尾
func injectSystemMessage(bodyBytes []byte, content string) ([]byte, error) {
	messages := gjson.GetBytes(bodyBytes, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return bodyBytes, fmt.Errorf("messages 不存在或不是数组")
	}

	arr := messages.Array()
	firstMsg := arr[0]

	// 如果第一条消息已是 system，追加内容
	if firstMsg.Get("role").String() == "system" {
		existingContent := firstMsg.Get("content").String()
		newContent := existingContent + "\n\n" + content
		result, err := sjson.SetBytes(bodyBytes, "messages.0.content", newContent)
		if err != nil {
			return bodyBytes, fmt.Errorf("更新 system content 失败: %w", err)
		}
		return result, nil
	}

	// 在 messages 数组最前面插入新 system 消息
	systemMsg := map[string]interface{}{
		"role":    "system",
		"content": content,
	}

	// 方法：把 messages 数组读出来，前面插入新消息，再写回去
	var bodyObj map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bodyObj); err != nil {
		return bodyBytes, fmt.Errorf("解析请求体失败: %w", err)
	}

	// messages 一定是 []interface{}
	origMessages := bodyObj["messages"].([]interface{})
	newMessages := make([]interface{}, 0, len(origMessages)+1)
	newMessages = append(newMessages, systemMsg)
	newMessages = append(newMessages, origMessages...)
	bodyObj["messages"] = newMessages

	modified, err := json.Marshal(bodyObj)
	if err != nil {
		return bodyBytes, fmt.Errorf("序列化修改后的请求体失败: %w", err)
	}

	return modified, nil
}
