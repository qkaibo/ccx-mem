package evolution

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// APIDeps holds dependencies for the evolution API.
type APIDeps struct {
	Store     *Store
	Tracker   *Tracker
	Analyzer  *Analyzer
	Auditor   *Auditor
	Patcher   *Patcher
	Publisher *Publisher
}

// SetupRoutes registers evolution API routes on the given Gin router group.
func SetupRoutes(rg *gin.RouterGroup, deps *APIDeps) {
	evo := rg.Group("/v2/evolution")

	// Prompt CRUD
	evo.GET("/prompts", listPrompts(deps))
	evo.POST("/prompts", createPrompt(deps))
	evo.GET("/prompts/:id", getPrompt(deps))
	evo.PUT("/prompts/:id", updatePrompt(deps))
	evo.DELETE("/prompts/:id", deletePrompt(deps))

	// Trace
	evo.POST("/traces", recordTrace(deps))
	evo.GET("/traces", listTraces(deps))

	// Defects
	evo.GET("/defects", listDefects(deps))

	// Analysis
	evo.POST("/analyze", triggerAnalyze(deps))

	// Audit
	evo.POST("/audit", triggerAudit(deps))

	// Skill CRUD
	evo.GET("/skills", listSkills(deps))
	evo.POST("/skills", createSkill(deps))
	evo.GET("/skills/:id", getSkill(deps))
	evo.PUT("/skills/:id", updateSkill(deps))
	evo.DELETE("/skills/:id", deleteSkill(deps))

	// Shared Learning
	evo.GET("/shared-learning", listSharedLearning(deps))

	// Publish
	evo.POST("/publish/defect/:id", publishDefect(deps))
	evo.POST("/publish/patch/:promptId", publishPatch(deps))
	evo.POST("/publish/audit/:id", publishAudit(deps))

	// Search OpenViking
	evo.POST("/search", searchOpenViking(deps))
}

// --- Handlers ---

func listPrompts(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		prompts, err := deps.Store.ListPrompts(limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, prompts)
	}
}

func createPrompt(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p Prompt
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		id, err := deps.Store.CreatePrompt(&p)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

func getPrompt(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		p, err := deps.Store.GetPrompt(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func updatePrompt(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var p Prompt
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := deps.Store.UpdatePrompt(id, &p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

func deletePrompt(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := deps.Store.DeletePrompt(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

func recordTrace(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var t ExecutionTrace
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if t.CreatedAt.IsZero() {
			t.CreatedAt = time.Now()
		}
		id, err := deps.Tracker.RecordTrace(&t)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

func listTraces(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		traces, err := deps.Tracker.GetRecentTraces(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, traces)
	}
}

func listDefects(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		defects, err := deps.Store.ListDefects(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, defects)
	}
}

func triggerAnalyze(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := deps.Analyzer.Analyze()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func triggerAudit(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PromptID   int64  `json:"prompt_id"`
			NewContent string `json:"new_content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := deps.Auditor.Audit(req.PromptID, req.NewContent)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// --- Skill CRUD Handlers ---

func listSkills(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		category := c.DefaultQuery("category", "")
		status := c.DefaultQuery("status", "")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		skills, err := deps.Store.ListSkills(category, status, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, skills)
	}
}

func createSkill(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var s Skill
		if err := c.ShouldBindJSON(&s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		id, err := deps.Store.CreateSkill(&s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

func getSkill(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		s, err := deps.Store.GetSkill(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

func updateSkill(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var s Skill
		if err := c.ShouldBindJSON(&s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := deps.Store.UpdateSkill(id, &s); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

func deleteSkill(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := deps.Store.DeleteSkill(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// --- Shared Learning Handlers ---

func listSharedLearning(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		records, err := deps.Store.ListSharedLearning(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, records)
	}
}

// --- Publish Handlers ---

func publishDefect(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		defect, err := deps.Store.GetDefect(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "defect not found"})
			return
		}
		rec, err := deps.Publisher.PublishDefect(defect)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rec)
	}
}

func publishPatch(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		promptID, err := strconv.ParseInt(c.Param("promptId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid promptId"})
			return
		}
		var req struct {
			NewVersion string `json:"new_version"`
			NewContent string `json:"new_content"`
			Reason     string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		patch := &Patch{
			PromptID:   promptID,
			NewVersion: req.NewVersion,
			NewContent: req.NewContent,
			Reason:     req.Reason,
			CreatedAt:  time.Now(),
		}
		rec, err := deps.Publisher.PublishPatch(patch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rec)
	}
}

func publishAudit(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		logs, err := deps.Store.ListAuditLogs(100)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var audit *AuditLog
		for _, l := range logs {
			if l.ID == id {
				audit = l
				break
			}
		}
		if audit == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit not found"})
			return
		}
		rec, err := deps.Publisher.PublishAuditResult(audit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rec)
	}
}

// --- Search Handler ---

func searchOpenViking(deps *APIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Query     string `json:"query"`
			TargetURI string `json:"target_uri"`
			Limit     int    `json:"limit"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := deps.Publisher.SearchOpenViking(req.Query, req.TargetURI, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result})
	}
}
