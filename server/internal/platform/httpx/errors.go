// Package httpx chứa định dạng lỗi chuẩn, response helper và middleware cho Gin.
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError là lỗi có mã ổn định để client xử lý. Mã lấy theo contracts/errors.yaml.
type AppError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Err     error          `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func (e *AppError) WithDetails(d map[string]any) *AppError {
	cp := *e
	cp.Details = d
	return &cp
}

func (e *AppError) Wrap(err error) *AppError {
	cp := *e
	cp.Err = err
	return &cp
}

var (
	ErrBadRequest       = NewError(http.StatusBadRequest, "bad_request", "Yêu cầu không hợp lệ")
	ErrValidation       = NewError(http.StatusUnprocessableEntity, "validation_failed", "Dữ liệu không hợp lệ")
	ErrUnauthorized     = NewError(http.StatusUnauthorized, "unauthorized", "Chưa đăng nhập")
	ErrForbidden        = NewError(http.StatusForbidden, "forbidden", "Không có quyền")
	ErrNotFound         = NewError(http.StatusNotFound, "not_found", "Không tìm thấy")
	ErrMethodNotAllowed = NewError(http.StatusMethodNotAllowed, "method_not_allowed", "Phương thức không được hỗ trợ")
	ErrConflict         = NewError(http.StatusConflict, "conflict", "Dữ liệu bị xung đột")
	ErrPayloadTooLarge  = NewError(http.StatusRequestEntityTooLarge, "payload_too_large", "Dữ liệu gửi lên quá lớn")
	ErrTooManyRequests  = NewError(http.StatusTooManyRequests, "rate_limited", "Quá nhiều yêu cầu")
	ErrInternal         = NewError(http.StatusInternalServerError, "internal_error", "Lỗi hệ thống")
	ErrUnavailable      = NewError(http.StatusServiceUnavailable, "service_unavailable", "Dịch vụ tạm thời không sẵn sàng")
)

type errorBody struct {
	Error struct {
		*AppError
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

// Error ghi lỗi vào context; ErrorHandler middleware sẽ render.
func Error(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

func writeError(c *gin.Context, appErr *AppError) {
	var body errorBody
	body.Error.AppError = appErr
	body.Error.RequestID = RequestIDFrom(c)
	c.AbortWithStatusJSON(appErr.Status, body)
}

func toAppError(err error) *AppError {
	if appErr, ok := errors.AsType[*AppError](err); ok {
		return appErr
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return ErrPayloadTooLarge.Wrap(err)
	}
	return ErrInternal.Wrap(err)
}
