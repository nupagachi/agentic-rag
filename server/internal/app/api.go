package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/dig"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

type apiDeps struct {
	dig.In

	Config    *config.Config
	Log       *logrus.Logger
	Server    *http.Server
	Lifecycle *lifecycle.Lifecycle
}

// RunAPI chạy HTTP server đến khi ctx bị hủy (SIGINT/SIGTERM) rồi tắt êm.
func RunAPI(ctx context.Context) error {
	c, err := BuildContainer()
	if err != nil {
		return err
	}
	// RootCause bỏ lớp bọc của dig để lỗi config/kết nối đọc được ngay.
	return dig.RootCause(c.Invoke(func(d apiDeps) error {
		var lc net.ListenConfig
		ln, err := lc.Listen(ctx, "tcp", d.Config.HTTP.Addr)
		if err != nil {
			return err
		}
		d.Log.WithFields(logrus.Fields{"addr": ln.Addr().String(), "env": d.Config.Env}).Info("api listening")
		return serve(ctx, d.Server, ln, d.Lifecycle, d.Config.HTTP, d.Log)
	}))
}

// serve phục vụ trên ln và khi ctx bị hủy thì tắt theo thứ tự:
//  1. lc.Shutdown(): /readyz trả 503, stream SSE và task nền nhận tín hiệu dừng.
//  2. Chờ DrainDelay để Nginx/LB kịp ngừng gửi request mới.
//  3. srv.Shutdown: đóng listener, chờ request đang chạy; quá ShutdownTimeout thì cắt cứng.
//  4. lc.Stop: chờ task nền, đóng Redis, Postgres (LIFO).
func serve(ctx context.Context, srv *http.Server, ln net.Listener, lc *lifecycle.Lifecycle, cfg config.HTTP, log *logrus.Logger) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		// Server chết ngoài ý muốn: vẫn dọn tài nguyên.
		stopCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return errors.Join(err, lc.Stop(stopCtx))
	case <-ctx.Done():
	}

	start := time.Now()
	log.WithField("timeout", cfg.ShutdownTimeout.String()).Info("shutdown started")
	lc.Shutdown()

	if cfg.DrainDelay > 0 {
		log.WithField("delay", cfg.DrainDelay.String()).Info("draining: waiting for load balancer")
		time.Sleep(cfg.DrainDelay)
	}

	// Timeout tính sau drain delay, dùng chung cho HTTP và đóng tài nguyên.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	var errs []error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Warn("http shutdown timed out, closing remaining connections")
		errs = append(errs, err, srv.Close())
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		errs = append(errs, err)
	}

	// Đóng tài nguyên vẫn cần ít thời gian kể cả khi HTTP đã dùng hết timeout.
	stopCtx := shutdownCtx
	if shutdownCtx.Err() != nil {
		var stopCancel context.CancelFunc
		stopCtx, stopCancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
	}
	if err := lc.Stop(stopCtx); err != nil {
		errs = append(errs, err)
	}

	err := errors.Join(errs...)
	entry := log.WithField("duration_ms", time.Since(start).Milliseconds())
	if err != nil {
		entry.WithError(err).Error("shutdown finished with errors")
	} else {
		entry.Info("shutdown complete")
	}
	return err
}
