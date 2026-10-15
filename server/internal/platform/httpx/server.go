package httpx

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.uber.org/dig"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
)

// Routes là thứ mỗi domain cung cấp vào dig group "routes" để tự đăng ký endpoint.
type Routes interface {
	Register(r *gin.Engine)
}

type RouterParams struct {
	dig.In

	Config *config.Config
	Log    *logrus.Logger
	Routes []Routes `group:"routes"`
}

// quietPaths là endpoint probe, log ở mức debug khi thành công.
var quietPaths = []string{"/healthz", "/readyz"}

func NewRouter(p RouterParams) (*gin.Engine, error) {
	if p.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.ContextWithFallback = true
	// nil (mặc định): không tin proxy nào, ClientIP là IP kết nối trực tiếp.
	if err := r.SetTrustedProxies(p.Config.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("HTTP_TRUSTED_PROXIES: %w", err)
	}

	r.Use(
		RequestID(),
		Logger(p.Log, quietPaths...),
		Recovery(),
		ErrorHandler(),
		MaxBodyBytes(p.Config.HTTP.MaxBodyBytes),
	)
	r.NoRoute(NoRoute)
	r.NoMethod(NoMethod)

	for _, rt := range p.Routes {
		rt.Register(r)
	}
	return r, nil
}

func NewServer(cfg *config.Config, r *gin.Engine) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}
}
