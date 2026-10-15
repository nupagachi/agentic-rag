//go:build ignore

// Chép contracts/openapi.yaml vào package này. Chạy qua: go generate ./internal/apidocs
package main

import (
	"log"
	"os"
)

func main() {
	data, err := os.ReadFile("../../../contracts/openapi.yaml")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("openapi.yaml", data, 0o644); err != nil {
		log.Fatal(err)
	}
}
