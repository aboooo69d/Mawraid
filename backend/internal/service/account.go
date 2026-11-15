package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/validation"
)

// ---------------------------------------------------------------------------
// Account status (SRS FR-AUTH-02 / FR-CON-04)
// ---------------------------------------------------------------------------

// AccountStatusResponse describes everything the client needs to route the UX.
type AccountStatusResponse struct {
	UserID             uint                      `json:"user_id"`
	Role               string                    `json:"role"`
	AccountStatus      models.AccountStatus      `json:"account_status"`
	PhoneVerified      bool                      `json:"phone_verified"`
	VerificationStatus models.VerificationStatus `json:"verification_status"`
	// CanLogin mirrors the SRS gating rules.
	CanLogin bool `json:"can_login"`
	// PendingDocuments is the count of documents waiting for admin review.
	PendingDocuments int `json:"pending_documents"`
	// Dashboard destination after login (SRS FR-AUTH-02).
	Dashboard string `json:"dashboard"`
	// NextStep is a machine hint for the frontend ("verify_otp",
	// "wait_approval", "upload_documents", "ready").
	NextStep string `json:"next_step"`
}

// GetAccountStatus returns the current account state.
func (s *Service) GetAccountStatus(ctx context.Context, userID uint) (AccountStatusResponse, error) {
	var user models.User
	if err := s.DB.WithContext(ctx).Preload("Role").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AccountStatusResponse{}, apierr.ErrNotFoundResource("Account")
		}
		return AccountStatusResponse{}, apierr.Internal("Could not load the account.", "تعذر تحميل الحساب.")
	}

	var pendingDocs int64
	_ = s.DB.WithContext(ctx).Model(&models.IdentityDocument{}).
		Where("user_id = ? AND status = ?", userID, "pending").Count(&pendingDocs).Error

	phoneVerified := user.PhoneVerifiedAt != nil
	resp := AccountStatusResponse{
		UserID: user.ID, Role: user.Role.Name, AccountStatus: user.Status,
		PhoneVerified: phoneVerified, VerificationStatus: user.VerificationStatus,
		CanLogin:         phoneVerified && user.Status == models.AccountStatusActive && user.IsActive,
		PendingDocuments: int(pendingDocs),
		Dashboard:        dashboardFor(user.Role.Name),
	}
	switch {
	case !phoneVerified:
		resp.NextStep = "verify_otp"
	case user.VerificationStatus == models.VerificationUnverified:
		resp.NextStep = "upload_documents"
	case user.VerificationStatus == models.VerificationPendingReview || pendingDocs > 0:
		resp.NextStep = "wait_approval"
	default:
		resp.NextStep = "ready"
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// Consumer identity verification (SRS FR-CON-01/05)
// ---------------------------------------------------------------------------

// SubmitConsumerVerification updates the identity number and moves the
// account into document review (verification_status = pending_review).
// It is used at first submission and again after a rejection.
func (s *Service) SubmitConsumerVerification(ctx context.Context, userID uint, idNumber string) error {
	var v validation.Violations
	id := validation.SanitizeText(idNumber)
	v.Required("id_number", id)
	v.TextLength("id_number", id, 4, 40)
	if err := v.Err(); err != nil {
		return err
	}

	var user models.User
	if err := s.DB.WithContext(ctx).First(&user, userID).Error; err != nil {
		return apierr.ErrNotFoundResource("Account")
	}
	// Duplicate id check (SRS FR-CON-02) excluding the current user.
	var dup uint64
	if err := s.DB.WithContext(ctx).
		Raw(`SELECT count(*) FROM users WHERE id_number = ? AND id <> ?`, id, userID).
		Scan(&dup).Error; err != nil {
		return apierr.Internal("Could not verify the data.", "تعذر التحقق من البيانات.")
	}
	if dup > 0 {
		return apierr.ErrDuplicateAccount()
	}

	updates := map[string]any{
		"id_number":           id,
		"verification_status": models.VerificationPendingReview,
		"updated_at":          s.Now(),
	}
	if err := s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return apierr.Internal("Could not save the verification data.", "تعذر حفظ بيانات التحقق.")
	}
	s.audit(ctx, &userID, &userID, "account.verification.submitted", "", nil)
	return nil
}

// ---------------------------------------------------------------------------
// Identity documents (SRS FR-CON-01/05, NFR-USA-01)
// ---------------------------------------------------------------------------

// AllowedDocumentTypes / allowed content types (SRS NFR-USA-01: JPG, PNG,
// PDF, max 5 MB).
const (
	DocTypeNationalID        = "national_id"
	DocTypeCommercialLicense = "commercial_license"
	DocTypeOther             = "other"
)

var allowedDocTypes = map[string]bool{
	DocTypeNationalID: true, DocTypeCommercialLicense: true, DocTypeOther: true,
}

// allowedContentTypes — validated against the sniffed magic bytes, not just
// the client-provided header.
var allowedContentTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"application/pdf": true,
}

// DocumentMeta is the public view of an uploaded document.
type DocumentMeta struct {
	ID           uint    `json:"id"`
	DocumentType string  `json:"document_type"`
	OriginalName string  `json:"original_name"`
	ContentType  string  `json:"content_type"`
	SizeBytes    int64   `json:"size_bytes"`
	Status       string  `json:"status"`
	ReviewNote   *string `json:"review_note,omitempty"`
	ReviewedAt   *string `json:"reviewed_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// SaveDocument stores an uploaded identity document for the user.
// The file is written under UPLOAD_DIR/<user>/<uuid>.<ext> with 0600
// permissions and is never exposed through a public static route; retrieval
// goes through the authenticated download endpoint.
func (s *Service) SaveDocument(ctx context.Context, userID uint, documentType, originalName, contentType string, content io.Reader) (DocumentMeta, error) {
	if !allowedDocTypes[documentType] {
		return DocumentMeta{}, apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "document_type", Message: "Unsupported document type. / نوع الوثيقة غير مدعوم.",
		}})
	}
	if originalName == "" || len(originalName) > 255 {
		return DocumentMeta{}, apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "file", Message: "Invalid file name. / اسم الملف غير صالح.",
		}})
	}
	// Strip any path components from the client-provided name.
	originalName = filepath.Base(filepath.Clean(originalName))

	maxSize := s.Cfg.Upload.MaxSizeBytes
	limited := io.LimitReader(content, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return DocumentMeta{}, apierr.BadRequest("UPLOAD_READ_FAILED", "Could not read the uploaded file.", "تعذر قراءة الملف المرفوع.")
	}
	if int64(len(data)) > maxSize {
		return DocumentMeta{}, apierr.ErrValidation([]apierr.FieldViolation{{
			Field:   "file",
			Message: fmt.Sprintf("File exceeds the %d MB limit. / حجم الملف يتجاوز الحد المسموح (%d ميجابايت).", maxSize/(1024*1024), maxSize/(1024*1024)),
		}})
	}
	if len(data) == 0 {
		return DocumentMeta{}, apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "file", Message: "The uploaded file is empty. / الملف المرفوع فارغ.",
		}})
	}

	// Sniff the real content type from magic bytes (NFR-USA-01 + security).
	sniffed := sniffContentType(data)
	if !allowedContentTypes[sniffed] {
		return DocumentMeta{}, apierr.ErrValidation([]apierr.FieldViolation{{
			Field:   "file",
			Message: "Only JPG, PNG or PDF files are accepted. / يُقبل فقط ملفات JPG أو PNG أو PDF.",
		}})
	}
	if contentType != "" {
		// Cross-check the declared type when provided.
		if ext, _ := mime.ExtensionsByType(contentType); len(ext) > 0 {
			if mimeFromExt := mime.TypeByExtension(ext[0]); mimeFromExt != sniffed && !(sniffed == "image/jpeg" && strings.HasPrefix(mimeFromExt, "image/jpeg")) {
				// Mismatch is not fatal when sniffed type is allowed; log only.
				s.Log.WarnContext(ctx, "upload declared content-type mismatch", "declared", contentType, "sniffed", sniffed)
			}
		}
	}

	ext := ".bin"
	switch sniffed {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "application/pdf":
		ext = ".pdf"
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	// De-duplicate identical content (same sha) for the same user.
	var existing models.IdentityDocument
	if err := s.DB.WithContext(ctx).
		Where("user_id = ? AND sha256 = ?", userID, sha).
		First(&existing).Error; err == nil && existing.Status != "rejected" {
		return toDocumentMeta(&existing), nil
	}

	fname := fmt.Sprintf("%d_%s%s", userID, randomHex(16), ext)
	relPath := filepath.ToSlash(filepath.Join(fmt.Sprintf("%d", userID), fname))
	absDir := filepath.Join(s.Cfg.Upload.Dir, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return DocumentMeta{}, apierr.Internal("Could not store the document.", "تعذر حفظ الوثيقة.")
	}
	absPath := filepath.Join(absDir, fname)
	if err := os.WriteFile(absPath, data, 0o600); err != nil {
		return DocumentMeta{}, apierr.Internal("Could not store the document.", "تعذر حفظ الوثيقة.")
	}

	doc := models.IdentityDocument{
		UserID: userID, DocumentType: documentType,
		FilePath: relPath, OriginalName: originalName,
		ContentType: sniffed, SizeBytes: int64(len(data)), Sha256: sha,
		Status: "pending",
	}
	if err := s.DB.WithContext(ctx).Create(&doc).Error; err != nil {
		_ = os.Remove(absPath)
		return DocumentMeta{}, apierr.Internal("Could not register the document.", "تعذر تسجيل الوثيقة.")
	}

	// Entering review: FR-CON-05 sends documents to the admin/AI pipeline.
	_ = s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Update("verification_status", models.VerificationPendingReview).Error
	s.audit(ctx, &userID, &userID, "account.document.uploaded", documentType, nil)

	return toDocumentMeta(&doc), nil
}

// ListDocuments returns the caller's documents (or an admin's view of any
// user's documents when userID is 0 and forAdmin is set).
func (s *Service) ListDocuments(ctx context.Context, userID uint) ([]DocumentMeta, error) {
	var docs []models.IdentityDocument
	if err := s.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").Find(&docs).Error; err != nil {
		return nil, apierr.Internal("Could not load documents.", "تعذر تحميل الوثائق.")
	}
	out := make([]DocumentMeta, 0, len(docs))
	for i := range docs {
		out = append(out, toDocumentMeta(&docs[i]))
	}
	return out, nil
}

// GetDocumentForDownload returns the document row when the caller may read it
// (owner or admin), plus its absolute filesystem path.
func (s *Service) GetDocumentForDownload(ctx context.Context, callerID uint, callerRole string, docID uint) (models.IdentityDocument, string, error) {
	var doc models.IdentityDocument
	if err := s.DB.WithContext(ctx).First(&doc, docID).Error; err != nil {
		return doc, "", apierr.ErrNotFoundResource("Document")
	}
	if callerRole != models.RoleAdmin && doc.UserID != callerID {
		return doc, "", apierr.ErrForbiddenRole()
	}
	abs := filepath.Join(s.Cfg.Upload.Dir, filepath.FromSlash(doc.FilePath))
	// Containment check — never escape the upload root.
	root, err := filepath.Abs(s.Cfg.Upload.Dir)
	if err != nil {
		return doc, "", apierr.Internal("Could not open the document.", "تعذر فتح الوثيقة.")
	}
	resolved, err := filepath.Abs(abs)
	if err != nil || !strings.HasPrefix(resolved, root) {
		return doc, "", apierr.Internal("Could not open the document.", "تعذر فتح الوثيقة.")
	}
	if _, err := os.Stat(resolved); err != nil {
		return doc, "", apierr.ErrNotFoundResource("Document file")
	}
	return doc, resolved, nil
}

// ReassignPendingDocuments is a helper for admin flows (kept for future use).
func (s *Service) countPendingDocs(ctx context.Context, userID uint) int64 {
	var n int64
	_ = s.DB.WithContext(ctx).Model(&models.IdentityDocument{}).
		Where("user_id = ? AND status = ?", userID, "pending").Count(&n).Error
	return n
}

// --- helpers ----------------------------------------------------------------

func toDocumentMeta(d *models.IdentityDocument) DocumentMeta {
	m := DocumentMeta{
		ID: d.ID, DocumentType: d.DocumentType, OriginalName: d.OriginalName,
		ContentType: d.ContentType, SizeBytes: d.SizeBytes, Status: d.Status,
		ReviewNote: d.ReviewNote, CreatedAt: d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if d.ReviewedAt != nil {
		s := d.ReviewedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		m.ReviewedAt = &s
	}
	return m
}

// sniffContentType detects JPEG/PNG/PDF from magic bytes.
func sniffContentType(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 8 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 &&
		b[4] == 0x0D && b[5] == 0x0A && b[6] == 0x1A && b[7] == 0x0A:
		return "image/png"
	case len(b) >= 4 && b[0] == '%' && b[1] == 'P' && b[2] == 'D' && b[3] == 'F':
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func randomHex(n int) string {
	t, err := randomToken()
	if err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	if len(t) > n {
		return t[:n]
	}
	return t
}
