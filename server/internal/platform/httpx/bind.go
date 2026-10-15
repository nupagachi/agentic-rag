package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func init() {
	// Báo lỗi theo tên trường JSON (snake_case) thay vì tên field Go.
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(jsonFieldName)
	}
}

func jsonFieldName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	switch name {
	case "-":
		return ""
	case "":
		return f.Name
	}
	return name
}

// BindJSON đọc body JSON vào dst rồi validate theo tag `binding`. Lỗi trả về
// đã đúng mã chuẩn (bad_request / validation_failed / payload_too_large):
//
//	var req CreateKBRequest
//	if err := httpx.BindJSON(c, &req); err != nil {
//		httpx.Error(c, err)
//		return
//	}
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return bindError(err)
	}
	return nil
}

// BindQuery giống BindJSON nhưng đọc query string (tag `form`).
func BindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		return bindError(err)
	}
	return nil
}

func bindError(err error) *AppError {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return ErrPayloadTooLarge.Wrap(err)
	}
	if ves, ok := errors.AsType[validator.ValidationErrors](err); ok {
		fields := make(map[string]string, len(ves))
		for _, fe := range ves {
			fields[fieldPath(fe)] = fieldMessage(fe)
		}
		return ErrValidation.WithDetails(map[string]any{"fields": fields}).Wrap(err)
	}
	if te, ok := errors.AsType[*json.UnmarshalTypeError](err); ok && te.Field != "" {
		fields := map[string]string{te.Field: "Sai kiểu dữ liệu, cần " + te.Type.String()}
		return ErrValidation.WithDetails(map[string]any{"fields": fields}).Wrap(err)
	}
	return ErrBadRequest.Wrap(err)
}

// fieldPath bỏ tên struct gốc: "CreateReq.owner.email" → "owner.email".
func fieldPath(fe validator.FieldError) string {
	if _, rest, ok := strings.Cut(fe.Namespace(), "."); ok {
		return rest
	}
	return fe.Field()
}

func fieldMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "Bắt buộc"
	case "email":
		return "Email không hợp lệ"
	case "uuid", "uuid4":
		return "ID không hợp lệ"
	case "url", "http_url":
		return "URL không hợp lệ"
	case "oneof":
		return "Phải là một trong: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "min", "gte":
		if fe.Kind() == reflect.String || fe.Kind() == reflect.Slice || fe.Kind() == reflect.Map {
			return fmt.Sprintf("Tối thiểu %s phần tử/ký tự", fe.Param())
		}
		return "Tối thiểu " + fe.Param()
	case "max", "lte":
		if fe.Kind() == reflect.String || fe.Kind() == reflect.Slice || fe.Kind() == reflect.Map {
			return fmt.Sprintf("Tối đa %s phần tử/ký tự", fe.Param())
		}
		return "Tối đa " + fe.Param()
	case "len":
		return "Độ dài phải là " + fe.Param()
	}
	return "Không hợp lệ (" + fe.Tag() + ")"
}
