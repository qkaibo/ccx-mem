package memory

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/BenedictKing/ccx/internal/evolution"
)

// TraceReader 执行轨迹读取接口（解耦 dreaming 与 evolution 包）
type TraceReader interface {
	GetRecentTraces(limit int) ([]*evolution.ExecutionTrace, error)
}

// DreamerConfig 梦境提取器配置
type DreamerConfig struct {
	MaxTraces int
}

// Dreamer 后台梦境提取器 — 分析执行轨迹自动提取记忆
// 设计来源: Letta Dreaming + Hermes agent-skill-evolution-v2
type Dreamer struct {
	traceReader TraceReader
	store       *Store
	maxTraces   int
}

// NewDreamer 创建梦境提取器
func NewDreamer(traceReader TraceReader, store *Store, cfg *DreamerConfig) *Dreamer {
	maxTraces := 100
	if cfg != nil && cfg.MaxTraces > 0 {
		maxTraces = cfg.MaxTraces
	}
	return &Dreamer{
		traceReader: traceReader,
		store:       store,
		maxTraces:   maxTraces,
	}
}

// Run 运行周期性梦境提取
func (d *Dreamer) Run(ctx context.Context, interval time.Duration) {
	if d.traceReader == nil {
		log.Printf("[Dreamer] 轨迹读取器未设置，跳过梦境提取")
		return
	}

	log.Printf("[Dreamer] 梦境提取已启动 (interval=%v, maxTraces=%d)", interval, d.maxTraces)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Dreamer] 梦境提取已停止")
			return
		case <-ticker.C:
			d.dreamCycle()
		}
	}
}

// DreamOnce 手动触发一次梦境提取（API 用）
func (d *Dreamer) DreamOnce() (int, error) {
	return d.dreamCycle()
}

func (d *Dreamer) dreamCycle() (int, error) {
	traces, err := d.traceReader.GetRecentTraces(d.maxTraces)
	if err != nil {
		log.Printf("[Dreamer] 获取轨迹失败: %v", err)
		return 0, err
	}

	if len(traces) == 0 {
		return 0, nil
	}

	patterns := d.extractPatterns(traces)
	if len(patterns) == 0 {
		return 0, nil
	}

	created := 0
	for _, p := range patterns {
		record := &MemoryRecord{
			Content: p.Content,
			Layer:   "indexed",
			UserID:  p.UserID,
			Tags:    strings.Join(p.Tags, ","),
			Source:  "auto-extracted",
		}

		id, err := d.store.InsertMemory(record)
		if err != nil {
			log.Printf("[Dreamer] 写入记忆失败: %v (content=%.80s)", err, p.Content)
			continue
		}

		log.Printf("[Dreamer] 自动提取记忆 #%d [%s]: %.120s", id, strings.Join(p.Tags, ","), p.Content)
		created++
	}

	return created, nil
}

type extractedPattern struct {
	Content string
	UserID  string
	Tags    []string
}

func (d *Dreamer) extractPatterns(traces []*evolution.ExecutionTrace) []extractedPattern {
	var patterns []extractedPattern

	userTraces := make(map[string][]*evolution.ExecutionTrace)
	for _, t := range traces {
		userTraces[t.UserID] = append(userTraces[t.UserID], t)
	}

	for userID, userTraceList := range userTraces {
		patterns = append(patterns, extractErrorPatterns(userID, userTraceList)...)
		patterns = append(patterns, extractSuccessRatePatterns(userID, userTraceList)...)
		patterns = append(patterns, extractKeywordPatterns(userID, userTraceList)...)
	}

	return patterns
}

func extractErrorPatterns(userID string, traces []*evolution.ExecutionTrace) []extractedPattern {
	errCounts := make(map[string]int)
	for _, t := range traces {
		if !t.Success && t.ErrorType != "" {
			errCounts[t.ErrorType]++
		}
	}

	var patterns []extractedPattern
	for errType, count := range errCounts {
		if count >= 3 {
			label := userLabel(userID)
			patterns = append(patterns, extractedPattern{
				Content: fmt.Sprintf("%s近期频繁遇到 %s 错误（最近批次中出现 %d 次），建议排查上游通道或模型配置", label, errType, count),
				UserID:  userID,
				Tags:    []string{"lesson", "error", errType},
			})
		}
	}
	return patterns
}

func extractSuccessRatePatterns(userID string, traces []*evolution.ExecutionTrace) []extractedPattern {
	if len(traces) < 5 {
		return nil
	}

	success := 0
	for _, t := range traces {
		if t.Success {
			success++
		}
	}
	rate := float64(success) / float64(len(traces)) * 100
	label := userLabel(userID)

	var patterns []extractedPattern
	if rate < 50 {
		patterns = append(patterns, extractedPattern{
			Content: fmt.Sprintf("%s请求成功率偏低：%.1f%%（最近 %d 次请求中 %d 次成功）", label, rate, len(traces), success),
			UserID:  userID,
			Tags:    []string{"fact", "reliability", "low-success-rate"},
		})
	} else if rate == 100 && len(traces) >= 10 {
		patterns = append(patterns, extractedPattern{
			Content: fmt.Sprintf("%s通道稳定，最近 %d 次请求全部成功", label, len(traces)),
			UserID:  userID,
			Tags:    []string{"fact", "reliability", "high-success-rate"},
		})
	}
	return patterns
}

func extractKeywordPatterns(userID string, traces []*evolution.ExecutionTrace) []extractedPattern {
	if len(traces) < 3 {
		return nil
	}

	wordFreq := make(map[string]int)
	for _, t := range traces {
		words := tokenize(t.RequestSummary)
		seen := make(map[string]bool)
		for _, w := range words {
			if !isStopWord(w) && len(w) >= 2 {
				if !seen[w] {
					wordFreq[w]++
					seen[w] = true
				}
			}
		}
	}

	type wordCount struct {
		word  string
		count int
	}
	var wcs []wordCount
	for w, c := range wordFreq {
		if c >= 3 {
			wcs = append(wcs, wordCount{w, c})
		}
	}
	sort.Slice(wcs, func(i, j int) bool {
		return wcs[i].count > wcs[j].count
	})

	var patterns []extractedPattern
	label := userLabel(userID)
	for i := 0; i < len(wcs) && i < 3; i++ {
		patterns = append(patterns, extractedPattern{
			Content: fmt.Sprintf("%s近期高频主题：%s（出现在 %d/%d 次请求中）", label, wcs[i].word, wcs[i].count, len(traces)),
			UserID:  userID,
			Tags:    []string{"fact", "topic", wcs[i].word},
		})
	}

	return patterns
}

func userLabel(userID string) string {
	if userID == "" {
		return "匿名用户"
	}
	return "用户 " + userID + " "
}

func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-')
	})
}
