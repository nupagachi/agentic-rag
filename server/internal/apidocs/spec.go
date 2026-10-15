package apidocs

import "github.com/getkin/kin-openapi/openapi3"

// LoadSpec parse spec nhúng; dùng cho test và (sau này) middleware validate request.
func LoadSpec() (*openapi3.T, error) {
	return openapi3.NewLoader().LoadFromData(Spec)
}
