// Package memory — 检索层：关键词提取与文本标准化
package memory

import (
	"strings"
	"unicode"
)

// ExtractKeywords 从文本中提取关键词，支持中英文混合
// 英文：split by whitespace + punctuation → lowercase → dedupe
// 中文：按 bigram 分词 → 去标点 → 去重
// 返回前 maxWords 个关键词，按原文顺序
func ExtractKeywords(text string, maxWords int) []string {
	if text == "" || maxWords <= 0 {
		return nil
	}

	normalized := normalizeForKeyword(text)
	words := splitMixed(normalized)

	seen := make(map[string]bool, len(words))
	result := make([]string, 0, maxWords)
	for _, w := range words {
		if len(w) < 2 {
			continue // 单字符跳过（无检索价值）
		}
		lower := strings.ToLower(w)
		if seen[lower] {
			continue
		}
		if isStopWord(lower) {
			continue
		}
		seen[lower] = true
		result = append(result, lower)
		if len(result) >= maxWords {
			break
		}
	}
	return result
}

// NormalizeContent 标准化记忆内容（存储前处理）
// 去除首尾空白，折叠连续空白行
func NormalizeContent(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var clean []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	return strings.Join(clean, "\n")
}

// ==========================================
//  内部辅助
// ==========================================

// normalizeForKeyword 移除标点、折叠空白（保留 CJK 字符）
func normalizeForKeyword(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if isPunct(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// splitMixed 混合分词：英文按空白，中文按 bigram
// 对于混合中英文的字段（如"用Go语言"），先按 CJK/ASCII 边界切分成段，再对大段做 bigram
func splitMixed(s string) []string {
	var result []string

	fields := strings.Fields(s)
	for _, f := range fields {
		segments := splitCJKASCIISegments(f)
		for _, seg := range segments {
			if isAllCJK(seg) {
				result = append(result, splitBigram(seg)...)
			} else {
				result = append(result, seg)
			}
		}
	}
	return result
}

// splitCJKASCIISegments 按 CJK/ASCII 边界切分字符串
// "用Go语言" → ["用", "Go", "语言"]
func splitCJKASCIISegments(s string) []string {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}

	var segments []string
	var buf []rune
	for i, r := range runes {
		isCJK := unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)
		// 第一个字符，或与上一个字符类型相同
		if i == 0 || (isCJK == (unicode.Is(unicode.Han, runes[i-1]) || unicode.Is(unicode.Hiragana, runes[i-1]) || unicode.Is(unicode.Katakana, runes[i-1]))) {
			buf = append(buf, r)
		} else {
			// 类型切换，保存当前段
			if len(buf) > 0 {
				segments = append(segments, string(buf))
			}
			buf = []rune{r}
		}
	}
	if len(buf) > 0 {
		segments = append(segments, string(buf))
	}
	return segments
}

// isAllCJK 是否全由 CJK 字符组成
func isAllCJK(s string) bool {
	for _, r := range s {
		if !unicode.Is(unicode.Han, r) && !unicode.Is(unicode.Hiragana, r) && !unicode.Is(unicode.Katakana, r) {
			return false
		}
	}
	return true
}

// splitBigram 中文 bigram 分词："你好世界" → ["你好","好世","世界"]
func splitBigram(s string) []string {
	runes := []rune(s)
	if len(runes) < 2 {
		return []string{s}
	}
	result := make([]string, 0, len(runes)-1)
	for i := 0; i < len(runes)-1; i++ {
		result = append(result, string(runes[i:i+2]))
	}
	return result
}

// isPunct 判断是否为标点（含 Unicode 标点、全角标点）
func isPunct(r rune) bool {
	if r <= 127 {
		return unicode.IsPunct(r) || r == '`' || r == '~' || r == '"' || r == '\''
	}
	return unicode.IsPunct(r)
}

// stopWords 常见停用词（中英混合）
var stopWords = map[string]bool{
	// 英文功能词
	"the": true, "a": true, "an": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true, "have": true,
	"has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "shall": true, "should": true, "may": true,
	"might": true, "can": true, "could": true, "must": true, "ought": true,
	"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
	"at": true, "by": true, "from": true, "as": true, "into": true, "through": true,
	"during": true, "before": true, "after": true, "above": true, "below": true,
	"between": true, "out": true, "off": true, "over": true, "under": true,
	"again": true, "further": true, "then": true, "once": true,
	"here": true, "there": true, "when": true, "where": true, "why": true, "how": true,
	"all": true, "both": true, "each": true, "few": true, "more": true,
	"most": true, "other": true, "some": true, "such": true,
	"no": true, "nor": true, "not": true, "only": true, "own": true,
	"same": true, "so": true, "than": true, "too": true, "very": true,
	"and": true, "but": true, "or": true, "if": true, "because": true,
	"about": true, "up": true, "just": true, "now": true,
	// 中文功能词
	"的": true, "了": true, "在": true, "是": true, "我": true, "有": true,
	"和": true, "就": true, "不": true, "人": true, "都": true, "一": true,
	"一个": true, "上": true, "也": true, "很": true, "到": true, "说": true,
	"要": true, "去": true, "你": true, "会": true, "着": true, "没有": true,
	"看": true, "好": true, "自己": true, "这": true, "他": true, "她": true,
	"它": true, "们": true, "那": true, "什么": true, "怎么": true, "哪": true,
	"为什么": true, "可以": true, "能": true, "还": true, "没": true,
	"这个": true, "那个": true, "这样": true, "那样": true,
}

// isStopWord 是否停用词
func isStopWord(s string) bool {
	return stopWords[s]
}
