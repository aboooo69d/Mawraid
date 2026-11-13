package storage_test

import (
	"context"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/najm-x/nova/pkg/storage"
)

// mockStorage implements storage.Storage in-memory for testing.
type mockStorage struct {
	saved map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{saved: make(map[string][]byte)}
}

func (m *mockStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	m.saved[key] = data
	return nil
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	delete(m.saved, key)
	return nil
}

// TestFileValidationAndUpload tests upload policies and validations.
func TestFileValidationAndUpload(t *testing.T) {
	key := make([]byte, 32)
	store := newMockStorage()
	uploader := storage.NewSecureUploader(store, key)

	t.Run("Reject File Exceeding Maximum Allowed Size (5MB)", func(t *testing.T) {
		oversizedHeader := &multipart.FileHeader{
			Filename: "large_image.jpg",
			Size:     6 << 20, // 6MB > 5MB
			Header:   make(textproto.MIMEHeader),
		}

		_, err := uploader.Upload(context.Background(), "test", oversizedHeader, storage.ImageTypes)
		if err != storage.ErrFileTooLarge {
			t.Fatalf("expected ErrFileTooLarge, got %v", err)
		}
	})

	t.Run("Reject File with Disallowed Mime Type", func(t *testing.T) {
		// ImageTypes allows only jpeg, png, webp
		allowed := storage.ImageTypes

		// Helper to verify mime types are properly whitelisted
		if storage.ImageAndPDF[len(storage.ImageAndPDF)-1] != "application/pdf" {
			t.Fatal("ImageAndPDF must include application/pdf")
		}

		for _, disallowed := range []string{"application/x-sh", "text/html", "application/octet-stream"} {
			found := false
			for _, m := range allowed {
				if m == disallowed {
					found = true
				}
			}
			if found {
				t.Fatalf("type %s should not be in allowed image list", disallowed)
			}
		}
	})
}

