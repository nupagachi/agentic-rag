package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/platform/logger"
)

const (
	HeaderRequestID = "X-Request-ID"
	ctxRequestID    = "request_id"

	maxRequestIDLen = 128
	// StatusClientClosedRequest (quy ước của Nginx): client ngắt trước khi có response.
	StatusClientClosedRequest = 499
)

// RequestID nhận X-Request-ID từ Nginx/FE nếu hợp lệ, không thì sinh mới.
// Chỉ nhận ký tự an toàn để không chèn được dòng giả vào log hay header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !validRequestID(id) {
			id = uuid.NewString()
		}
		c.Set(ctxRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		switch b := id[i]; {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9',
			b == '-', b == '_', b == '.', b == ':':
		default:
			return false
		}
	}
	return true
}

func RequestIDFrom(c *gin.Context) string { return c.GetString(ctxRequestID) }

// Logger gắn logger có request_id vào context của request và ghi 1 dòng log
// cho mỗi request. Request thành công tới quietPaths (healthcheck của Docker/LB,
// gọi vài giây một lần) ghi ở mức debug để không lấp log thật.
func Logger(log *logrus.Logger, quietPaths ...string) gin.HandlerFunc {
	quiet := make(map[string]bool, len(quietPaths))
	for _, p := range quietPaths {
		quiet[p] = true
	}
	return func(c *gin.Context) {
		start := time.Now()
		entry := log.WithField("request_id", RequestIDFrom(c))
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), entry))

		c.Next()

		status := c.Writer.Status()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		e := entry.WithFields(logrus.Fields{
			"method":      c.Request.Method,
			"path":        path,
			"status":      status,
			"duration_ms": time.Since(start).Milliseconds(),
			"client_ip":   c.ClientIP(),
			"bytes":       c.Writer.Size(),
		})
		switch {
		case status >= 500:
			e.Error("request")
		case status >= 400:
			e.Warn("request")
		case quiet[path]:
			e.Debug("request")
		default:
			e.Info("request")
		}
	}
}

// Log trả logger gắn request_id; dùng trong handler. Ở service/repo dùng
// logger.FromContext(ctx) với ctx là c.Request.Context().
func Log(c *gin.Context) *logrus.Entry { return logger.FromContext(c.Request.Context()) }

// MaxBodyBytes giới hạn kích thước body; vượt quá thì 413. n <= 0: không giới hạn.
func MaxBodyBytes(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if n <= 0 || c.Request.Body == nil || c.Request.Body == http.NoBody {
			c.Next()
			return
		}
		if c.Request.ContentLength > n {
			writeError(c, ErrPayloadTooLarge)
			return
		}
		// Body chunked không có Content-Length: chặn khi đọc, lỗi map sang 413 ở ErrorHandler/BindJSON.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// Recovery bắt panic, log stack và trả lỗi chuẩn 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			// http.ErrAbortHandler là cách chủ động hủy response (vd proxy): để net/http xử lý.
			if r == http.ErrAbortHandler { //nolint:errorlint // so sánh giá trị panic, không phải error chain
				panic(r)
			}
			if err, ok := r.(error); ok && isConnClosed(err) {
				Log(c).WithError(err).Warn("client connection closed")
				c.Abort()
				return
			}
			Log(c).WithField("stack", string(debug.Stack())).Errorf("panic: %v", r)
			if c.Writer.Written() {
				c.Abort() // header đã gửi, không thể đổi thành 500
				return
			}
			writeError(c, ErrInternal.Wrap(fmt.Errorf("panic: %v", r)))
		}()
		c.Next()
	}
}

func isConnClosed(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)
}

// ErrorHandler render lỗi cuối cùng mà handler đẩy vào c.Errors.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err

		// Client đã ngắt (đóng tab, hủy stream): không còn ai đọc, không phải lỗi server.
		if errors.Is(err, context.Canceled) && c.Request.Context().Err() != nil {
			Log(c).WithError(err).Debug("client canceled request")
			c.AbortWithStatus(StatusClientClosedRequest)
			return
		}

		appErr := toAppError(err)
		if appErr.Status >= http.StatusInternalServerError {
			Log(c).WithError(appErr).Error("handler error")
		}
		writeError(c, appErr)
	}
}

func NoRoute(c *gin.Context)  { writeError(c, ErrNotFound) }
func NoMethod(c *gin.Context) { writeError(c, ErrMethodNotAllowed) }
