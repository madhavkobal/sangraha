package s3

import (
	"testing"

	"github.com/madhavkobal/sangraha/internal/storage"
)

func TestIsMultipartNotFound(t *testing.T) {
	err := &storage.MultipartNotFoundError{UploadID: "test-id"}
	if !isMultipartNotFound(err) {
		t.Error("isMultipartNotFound should return true for *storage.MultipartNotFoundError")
	}
	if isMultipartNotFound(nil) {
		t.Error("isMultipartNotFound should return false for nil")
	}
}
