package evolution

import (
	"context"
	"log"
	"time"
)

// LoopConfig 进化循环配置
type LoopConfig struct {
	Interval  time.Duration
	AutoApply bool
}

// RunLoop 运行自进化分析循环（后台 goroutine）
func RunLoop(
	ctx context.Context,
	cfg LoopConfig,
	analyzer *Analyzer,
	store *Store,
	patcher *Patcher,
) {
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Minute
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	log.Printf("[Evolution-Loop] 自进化分析循环已启动 (间隔: %v, autoApply=%v)", cfg.Interval, cfg.AutoApply)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Evolution-Loop] 自进化分析循环已停止")
			return
		case <-ticker.C:
			runCycle(analyzer, store, patcher, cfg.AutoApply)
		}
	}
}

func runCycle(analyzer *Analyzer, store *Store, patcher *Patcher, autoApply bool) {
	result, err := analyzer.Analyze()
	if err != nil {
		log.Printf("[Evolution-Loop] 轨迹分析失败: %v", err)
		return
	}

	if result.DefectsCreated == 0 {
		log.Printf("[Evolution-Loop] 分析完成: %s", result.Summary)
		return
	}

	log.Printf("[Evolution-Loop] %s", result.Summary)

	for _, defectID := range result.DefectIDs {
		defect, err := store.GetDefect(defectID)
		if err != nil || defect == nil {
			log.Printf("[Evolution-Loop] 获取缺陷失败: id=%d, err=%v", defectID, err)
			continue
		}

		patch, err := patcher.GeneratePatchFromAnalysis(defect)
		if err != nil {
			log.Printf("[Evolution-Loop] 补丁生成失败: %v", err)
			continue
		}

		log.Printf("[Evolution-Loop] 补丁已生成: prompt=%d, reason=%s", patch.PromptID, patch.Reason)

		if !autoApply {
			log.Printf("[Evolution-Loop] 跳过自动应用 (autoApply=false)")
			continue
		}

		if err := patcher.ApplyPatch(patch, true); err != nil {
			log.Printf("[Evolution-Loop] 补丁应用失败: %v", err)
		} else {
			log.Printf("[Evolution-Loop] 补丁已自动应用: prompt=%d v=%s", patch.PromptID, patch.NewVersion)
		}
	}
}
