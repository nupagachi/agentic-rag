package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
)

type createReq struct {
	Name  string `json:"name" binding:"required,max=5"`
	Email string `json:"email" binding:"omitempty,email"`
	Owner struct {
		Role string `json:"role" binding:"required,oneof=owner admin"`
	} `json:"owner"`
	Size int `json:"size"`
}

func TestBindJSON(t *testing.T) {
	r := newRouter(t, config.HTTP{MaxBodyBytes: 256}, func(r *gin.Engine) {
		r.POST("/kb", func(c *gin.Context) {
			var req createReq
			if err := BindJSON(c, &req); err != nil {
				Error(c, err)
				return
			}
			c.Status(http.StatusCreated)
		})
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantFields map[string]string
	}{
		{name: "ok", body: `{"name":"kb","owner":{"role":"admin"}}`, wantStatus: http.StatusCreated},
		{name: "syntax", body: `{"name":`, wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "empty", body: ``, wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{
			name: "validation", body: `{"name":"too-long","email":"x","owner":{"role":"guest"}}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed",
			wantFields: map[string]string{"name": "Tối đa", "email": "Email", "owner.role": "owner, admin"},
		},
		{
			name: "wrong type", body: `{"name":"kb","size":"big","owner":{"role":"admin"}}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed",
			wantFields: map[string]string{"size": "Sai kiểu"},
		},
		{
			name: "too large", body: `{"name":"` + strings.Repeat("x", 300) + `"}`,
			wantStatus: http.StatusRequestEntityTooLarge, wantCode: "payload_too_large",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/kb", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.ContentLength = -1 // để "too large" đi qua nhánh BindJSON thay vì chặn sớm
			w, body := do(t, r, req)
			if w.Code != tt.wantStatus || body.Error.Code != tt.wantCode {
				t.Fatalf("got %d %q, want %d %q (%s)", w.Code, body.Error.Code, tt.wantStatus, tt.wantCode, w.Body.String())
			}
			if tt.wantFields == nil {
				return
			}
			fields, _ := body.Error.Details["fields"].(map[string]any)
			if len(fields) != len(tt.wantFields) {
				t.Fatalf("fields = %v, want keys of %v", fields, tt.wantFields)
			}
			for k, sub := range tt.wantFields {
				msg, _ := fields[k].(string)
				if !strings.Contains(msg, sub) {
					t.Errorf("fields[%q] = %q, want contains %q", k, msg, sub)
				}
			}
		})
	}
}
