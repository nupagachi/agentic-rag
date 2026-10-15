// Package health cung cấp /healthz (liveness) và /readyz (readiness).
//
// Phụ thuộc cần kiểm tra cho /readyz được provide vào dig group "health_checks"
// dưới dạng Check (xem app.BuildContainer), nên thêm Qdrant, MinIO... không phải sửa package này.
package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/dig"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

const checkTimeout = time.Second

// Check là một phụ thuộc /readyz phải gọi được thì instance mới nhận traffic.
type Check struct {
	Name string
	Ping func(context.Context) error
}

type Params struct {
	dig.In

	Config    *config.Config
	Lifecycle *lifecycle.Lifecycle
	Checks    []Check `group:"health_checks"`
}

type Handler struct {
	checks     []Check
	showErrors bool
	lc         *lifecycle.Lifecycle
}

func NewHandler(p Params) *Handler {
	return &Handler{lc: p.Lifecycle, showErrors: !p.Config.IsProduction(), checks: p.Checks}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz", h.live)
	r.GET("/readyz", h.ready)
}

func (h *Handler) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type checkResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ready chạy song song mọi check, mỗi check tối đa 1s. Khi đang tắt
// thì trả 503 ngay để LB ngừng gửi request mới.
func (h *Handler) ready(c *gin.Context) {
	if h.lc.ShuttingDown() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "shutting_down"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), checkTimeout)
	defer cancel()

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make(map[string]checkResult, len(h.checks))
		allOK   = true
	)
	for _, check := range h.checks {
		wg.Go(func() {
			res := checkResult{Status: "up"}
			if err := check.Ping(ctx); err != nil {
				res.Status = "down"
				if h.showErrors {
					res.Error = err.Error()
				}
			}
			mu.Lock()
			results[check.Name] = res
			if res.Status != "up" {
				allOK = false
			}
			mu.Unlock()
		})
	}
	wg.Wait()

	status, code := "ok", http.StatusOK
	if !allOK {
		status, code = "unavailable", http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"status": status, "checks": results})
}
