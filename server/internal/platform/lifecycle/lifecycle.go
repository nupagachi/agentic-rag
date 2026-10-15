// Package lifecycle quản lý quá trình tắt êm (graceful shutdown) của tiến trình.
//
// Thứ tự khi tắt:
//  1. Shutdown() đánh dấu đang tắt và hủy Context() → /readyz trả 503,
//     stream dài (SSE) và goroutine nền nhận tín hiệu dừng.
//  2. Caller tắt HTTP server (chờ request đang chạy).
//  3. Stop() chờ goroutine nền (Go) kết thúc, rồi chạy hook đóng tài nguyên
//     theo thứ tự ngược với lúc đăng ký (LIFO), mọi bước bị chặn bởi ctx.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

type hook struct {
	name string
	stop func(context.Context) error
}

type Lifecycle struct {
	log          *logrus.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	shuttingDown atomic.Bool

	mu    sync.Mutex
	hooks []hook
	wg    sync.WaitGroup
}

func New(log *logrus.Logger) *Lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &Lifecycle{log: log, ctx: ctx, cancel: cancel}
}

// OnStop đăng ký hàm đóng tài nguyên; hook đăng ký sau chạy trước.
func (l *Lifecycle) OnStop(name string, stop func(context.Context) error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook{name: name, stop: stop})
}

// Go chạy goroutine nền; ctx truyền vào bị hủy khi bắt đầu tắt,
// Stop() chờ goroutine trả về trước khi đóng tài nguyên.
func (l *Lifecycle) Go(name string, fn func(ctx context.Context)) {
	l.wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				l.log.WithField("task", name).Errorf("background task panic: %v", r)
			}
		}()
		fn(l.ctx)
	})
}

// Context bị hủy ngay khi bắt đầu tắt. Handler SSE/long-poll nên select thêm
// trên Context().Done() để kết thúc stream thay vì giữ server tới hết timeout.
func (l *Lifecycle) Context() context.Context { return l.ctx }

func (l *Lifecycle) ShuttingDown() bool { return l.shuttingDown.Load() }

// Shutdown đánh dấu bắt đầu tắt; gọi nhiều lần vô hại.
func (l *Lifecycle) Shutdown() {
	if l.shuttingDown.CompareAndSwap(false, true) {
		l.cancel()
	}
}

// Stop chờ goroutine nền rồi chạy các hook theo LIFO. Hook vẫn chạy dù ctx
// đã hết hạn (để còn đóng kết nối), lỗi được gom lại trả về.
func (l *Lifecycle) Stop(ctx context.Context) error {
	l.Shutdown()

	var errs []error
	done := make(chan struct{})
	go func() { l.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		errs = append(errs, fmt.Errorf("background tasks: %w", ctx.Err()))
	}

	l.mu.Lock()
	hooks := l.hooks
	l.hooks = nil
	l.mu.Unlock()

	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		if err := h.stop(ctx); err != nil {
			l.log.WithError(err).WithField("hook", h.name).Warn("stop hook failed")
			errs = append(errs, fmt.Errorf("%s: %w", h.name, err))
			continue
		}
		l.log.WithField("hook", h.name).Debug("stopped")
	}
	return errors.Join(errs...)
}
