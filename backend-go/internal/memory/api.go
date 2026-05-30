// Package memory — REST API 层
package memory

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/gin-gonic/gin"
)

// MemoryAPIDeps 记忆 API 依赖
type MemoryAPIDeps struct {
	Store        *Store
	Injector     *Injector
	EnvCfg       *config.EnvConfig
}

// SetupRoutes 注册记忆 API 路由到 /api/v2/memories
func SetupRoutes(r gin.IRouter, deps *MemoryAPIDeps) {
	g := r.Group("/v2/memories")
	{
		g.POST("", createMemory(deps))
		g.GET("/query", searchMemories(deps))
		g.GET("/:id", getMemory(deps))
		g.PUT("/:id", updateMemory(deps))
		g.DELETE("/:id", deleteMemory(deps))
	}
}

// ==========================================
//  Handlers
// ==========================================

type createMemoryRequest struct {
	Content string `json:"content"`
	Layer   string `json:"layer"`    // "core" | "indexed" (默认 "indexed")
	UserID  string `json:"user_id"`  // 可选，空 = 全局
	Tags    string `json:"tags"`     // 逗号分隔
	Source  string `json:"source"`   // "manual" | "auto-extracted" (默认 "manual")
}

func createMemory(deps *MemoryAPIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createMemoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.Content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
			return
		}

		// 标准化内容
		req.Content = NormalizeContent(req.Content)

		layer := req.Layer
		if layer == "" {
			layer = "indexed"
		}
		source := req.Source
		if source == "" {
			source = "manual"
		}

		rec := &MemoryRecord{
			Content: req.Content,
			Layer:   layer,
			UserID:  req.UserID,
			Tags:    req.Tags,
			Source:  source,
		}

		id, err := deps.Store.InsertMemory(rec)
		if err != nil {
			deps.log("createMemory", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create memory"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

func searchMemories(deps *MemoryAPIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Query("user_id")
		q := c.Query("q")
		limitStr := c.DefaultQuery("limit", "10")

		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		var records []MemoryRecord

		if q != "" {
			records, err = deps.Store.SearchMemories(userID, q, limit)
		} else {
			records, err = deps.Store.ListMemoriesByUser(userID, limit, 0)
		}

		if err != nil {
			deps.log("searchMemories", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}
		if records == nil {
			records = []MemoryRecord{}
		}

		c.JSON(http.StatusOK, records)
	}
}

func getMemory(deps *MemoryAPIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		rec, err := deps.Store.GetMemory(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, rec)
	}
}

func updateMemory(deps *MemoryAPIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var req createMemoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		// 获取现有记录
		existing, err := deps.Store.GetMemory(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// 按字段覆盖（只覆盖非空字段）
		if req.Content != "" {
			existing.Content = NormalizeContent(req.Content)
		}
		if req.Layer != "" {
			existing.Layer = req.Layer
		}
		if req.UserID != "" {
			existing.UserID = req.UserID
		}
		if req.Tags != "" {
			existing.Tags = req.Tags
		}
		if req.Source != "" {
			existing.Source = req.Source
		}

		if err := deps.Store.UpdateMemory(existing); err != nil {
			deps.log("updateMemory", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update memory"})
			return
		}

		c.JSON(http.StatusOK, existing)
	}
}

func deleteMemory(deps *MemoryAPIDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		if err := deps.Store.DeleteMemory(id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"deleted": true})
	}
}

// ==========================================
//  工具函数
// ==========================================

func parseID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func (deps *MemoryAPIDeps) log(op string, err error) {
	if deps.EnvCfg != nil && deps.EnvCfg.EnableRequestLogs {
		fmt.Printf("[Memory-API] %s: %v\n", op, err)
	}
}
