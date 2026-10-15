// Package apidocs phục vụ contracts/openapi.yaml và Swagger UI cho FE đọc.
//
// openapi.yaml trong thư mục này là bản sao của contracts/openapi.yaml (để nhúng
// bằng go:embed và build Docker với context ./server). Sửa file gốc rồi chạy:
//
//	go generate ./internal/apidocs
//
// TestSpecInSync sẽ fail nếu hai file lệch nhau.
package apidocs

//go:generate go run sync.go

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
	swgui "github.com/swaggest/swgui/v5"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
)

//go:embed openapi.yaml
var Spec []byte

const (
	SpecPath = "/api/openapi.yaml"
	UIPath   = "/api/docs"
)

type Handler struct {
	enabled bool
	ui      http.Handler
}

func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		enabled: cfg.APIDocs.Enabled,
		ui:      swgui.New("Agentic RAG API", SpecPath, UIPath+"/"),
	}
}

func (h *Handler) Register(r *gin.Engine) {
	if !h.enabled {
		return
	}
	r.GET(SpecPath, func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", Spec)
	})
	r.GET(UIPath, func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, UIPath+"/") })
	r.GET(UIPath+"/*any", gin.WrapH(h.ui))
}
