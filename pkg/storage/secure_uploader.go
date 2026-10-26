package storage

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/najm-x/nova/pkg/crypto"
)

const MaxFileSize = 5 << 20 // 5MB

var (
	ErrFileTooLarge    = errors.New("file is too large (max 5MB)")
	ErrFileTypeInvalid = errors.New("file type is not allowed")
	ErrFileEmpty       = errors.New("file is empty")
)

// Common allowed content types.
var (
	ImageTypes    = []string{"image/jpeg", "image/png", "image/webp"}
	ImageAndPDF   = []string{"image/jpeg", "image/png", "image/webp", "application/pdf"}
	extByMimeType = map[string]string{
		"image/jpeg":      "jpg",
		"image/png":       "png",
		"image/webp":      "webp",
		"application/pdf": "pdf",
	}
)

// SecureUploader validates a file, encrypts it (AES-256-GCM) and uploads it.
// The stored object is encrypted binary, so it cannot be viewed directly.
type SecureUploader struct {
	store Storage
	key   []byte
}

func NewSecureUploader(store Storage, encryptionKey []byte) *SecureUploader {
	return &SecureUploader{store: store, key: encryptionKey}
}

// Upload validates, encrypts and stores the file under folder and returns the
// object key (store this in the database, not a URL).
// The real content type is appended to the key name (e.g. consumers/id/<uuid>.jpg.enc).
func (u *SecureUploader) Upload(ctx context.Context, folder string, fh *multipart.FileHeader, allowed []string) (string, error) {
	if fh.Size > MaxFileSize {
		return "", ErrFileTooLarge
	}

	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read at most MaxFileSize+1 so a lying header cannot exhaust memory.
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", ErrFileEmpty
	}
	if len(data) > MaxFileSize {
		return "", ErrFileTooLarge
	}

	// Detect the real type from content, ignore client extension/header.
	mimeType := http.DetectContentType(data)
	if !contains(allowed, mimeType) {
		return "", ErrFileTypeInvalid
	}

	encrypted, err := crypto.Encrypt(u.key, data)
	if err != nil {
		return "", err
	}

	objectKey := strings.Trim(folder, "/") + "/" + uuid.NewString() + "." + extByMimeType[mimeType] + ".enc"
	if err := u.store.Put(ctx, objectKey, encrypted, "application/octet-stream"); err != nil {
		return "", err
	}
	return objectKey, nil
}

// Delete removes an uploaded object (used for cleanup if later steps fail).
func (u *SecureUploader) Delete(ctx context.Context, key string) error {
	return u.store.Delete(ctx, key)
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
