package apidocs

import (
	"bytes"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecInSync(t *testing.T) {
	src, err := os.ReadFile("../../../contracts/openapi.yaml")
	if err != nil {
		t.Skipf("contracts/openapi.yaml not found (build ngoài monorepo?): %v", err)
	}
	norm := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if !bytes.Equal(norm(src), norm(Spec)) {
		t.Fatal("internal/apidocs/openapi.yaml lệch contracts/openapi.yaml: chạy `go generate ./internal/apidocs`")
	}
}

func TestSpecValid(t *testing.T) {
	doc, err := LoadSpec()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(t.Context(), openapi3.EnableExamplesValidation()); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
}
