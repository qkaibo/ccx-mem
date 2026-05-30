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
	publisher *Publisher,
) {
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Minute
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	log.Printf("[Evolution-Loop] 自进化分析循环已启动 (间隔: %v, autoApply=%v, publisher=%v)", cfg.Interval, cfg.AutoApply, publisher != nil)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Evolution-Loop] 自进化分析循环已停止")
			return
		case <-ticker.C:
			runCycle(analyzer, store, patcher, publisher, cfg.AutoApply)
		}
	}
}

func runCycle(analyzer *Analyzer, store *Store, patcher *Patcher, publisher *Publisher, autoApply bool) {
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

		// 发布缺陷到 OpenViking
		if publisher != nil {
			if rec, err := publisher.PublishDefect(defect); err != nil {
				log.Printf("[Evolution-Loop] 发布缺陷失败: id=%d, err=%v", defectID, err)
			} else if rec != nil {
				log.Printf("[Evolution-Loop] 缺陷已发布: id=%d -> %s", defectID, rec.TargetURI)
			}
		}

		patch, err := patcher.GeneratePatchFromAnalysis(defect)
		if err != nil {
			log.Printf("[Evolution-Loop] 补丁生成失败: %v", err)
			continue
		}

		log.Printf("[Evolution-Loop] 补丁已生成: prompt=%d, reason=%s", patch.PromptID, patch.Reason)

		if autoApply {
			if err := patcher.ApplyPatch(patch, true); err != nil {
				log.Printf("[Evolution-Loop] 补丁应用失败: %v", err)
			} else {
				log.Printf("[Evolution-Loop] 补丁已自动应用: prompt=%d v=%s", patch.PromptID, patch.NewVersion)
			}
		}

		// 发布补丁到 OpenViking（无论是否自动应用，都记录补丁内容）
		if publisher != nil {
			if rec, err := publisher.PublishPatch(patch); err != nil {
				log.Printf("[Evolution-Loop] 发布补丁失败: prompt=%d, err=%v", patch.PromptID, err)
			} else if rec != nil {
				log.Printf("[Evolution-Loop] 补丁已发布: prompt=%d -> %s", patch.PromptID, rec.TargetURI)
			}
		}
	}
}
