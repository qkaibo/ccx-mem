package evolution

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// APIDeps holds dependencies for the evolution API.
type APIDeps struct {
	Store    *Store
	Tracker  *Tracker
	Analyzer *Analyzer
	Auditor  *Auditor
	Patcher  *Patcher
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
