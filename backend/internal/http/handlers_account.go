package http

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Account status (SRS FR-AUTH-02 / FR-CON-04)
// ---------------------------------------------------------------------------

func (s *Server) handleAccountStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.svc.GetAccountStatus(r.Context(), CurrentUserID(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, status)
}

// ---------------------------------------------------------------------------
// Consumer identity verification (SRS FR-CON-01/05)
// ---------------------------------------------------------------------------

func (s *Server) handleSubmitVerification(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDNumber string `json:"id_number"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	userID := CurrentUserID(r.Context())
	if err := s.svc.SubmitConsumerVerification(r.Context(), userID, in.IDNumber); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"verification_status": models.VerificationPendingReview,
		"next_step":           "upload_documents",
		"message":             "Identity data submitted. Upload your documents to continue.",
		"message_ar":          "تم إرسال بيانات الهوية. ارفع وثائقك للمتابعة.",
	})
}

// ---------------------------------------------------------------------------
// Identity documents (SRS FR-CON-01/05, NFR-USA-01: JPG/PNG/PDF ≤ 5 MB)
// ---------------------------------------------------------------------------

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := s.svc.ListDocuments(r.Context(), CurrentUserID(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"documents": docs})
}

// handleUploadDocument accepts multipart/form-data: "document_type" + "file".
func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	s.handleUploadDocumentFor(w, r, CurrentUserID(r.Context()))
}

// handleAdminUploadDocument lets an admin attach a document to any account
// (back-office path; same validation as the self-service upload).
func (s *Server) handleAdminUploadDocument(w http.ResponseWriter, r *http.Request) {
	targetID, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.handleUploadDocumentFor(w, r, targetID)
}

func (s *Server) handleUploadDocumentFor(w http.ResponseWriter, r *http.Request, userID uint) {
	file, header, docType, err := parseUpload(w, r, s.svc.Cfg.Upload.MaxSizeBytes, true)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	defer file.Close() //nolint:errcheck

	meta, err := s.svc.SaveDocument(r.Context(), userID, docType, header.Filename,
		header.Header.Get("Content-Type"), file)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, map[string]any{
		"document":   meta,
		"message":    "Document uploaded and queued for review.",
		"message_ar": "تم رفع الوثيقة وإرسالها للمراجعة.",
	})
}

// parseUpload parses one multipart file field (plus document_type when
// requireDocType is set — the provider logo upload has no document type).
func parseUpload(w http.ResponseWriter, r *http.Request, maxSize int64, requireDocType bool) (multipart.File, *multipart.FileHeader, string, error) {
	// Multipart overhead (boundaries, fields) gets 1 MB of slack over the file cap.
	r.Body = http.MaxBytesReader(w, r.Body, maxSize+(1<<20))
	if err := r.ParseMultipartForm(maxSize); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, nil, "", apierr.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE",
				fmt.Sprintf("Upload exceeds the %d MB limit.", maxSize/(1024*1024)),
				fmt.Sprintf("حجم الملف يتجاوز الحد المسموح (%d ميجابايت).", maxSize/(1024*1024)))
		}
		return nil, nil, "", apierr.BadRequest("MULTIPART_INVALID",
			"Expected multipart/form-data with a \"file\" field.",
			"الشكل المتوقع multipart/form-data يحتوي حقل file.")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, nil, "", apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "file", Message: "A file field is required. / حقل الملف مطلوب.",
		}})
	}
	docType := r.FormValue("document_type")
	if requireDocType && docType == "" {
		file.Close() //nolint:errcheck
		return nil, nil, "", apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "document_type", Message: "This field is required. / هذا الحقل مطلوب.",
		}})
	}
	return file, header, docType, nil
}

// handleDownloadDocument streams a stored document to its owner or an admin.
// Files are never exposed through a static public route (NFR-SEC-05).
func (s *Server) handleDownloadDocument(w http.ResponseWriter, r *http.Request) {
	docID, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	doc, absPath, err := s.svc.GetDocumentForDownload(r.Context(),
		CurrentUserID(r.Context()), CurrentUserRole(r.Context()), docID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	f, err := os.Open(absPath)
	if err != nil {
		s.writeError(w, r, apierr.ErrNotFoundResource("Document file"))
		return
	}
	defer f.Close() //nolint:errcheck

	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(doc.SizeBytes, 10))
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(doc.OriginalName)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	if id := RequestID(r.Context()); id != "" {
		w.Header().Set("X-Request-Id", id)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// sanitizeFilename keeps only safe characters for the Content-Disposition.
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_', r == ' ', r >= 0x0600 && r <= 0x06FF:
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "document"
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// Notifications (ERD table Notification)
// ---------------------------------------------------------------------------

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := CurrentUserID(r.Context())
	limit, offset := pagination(r)

	var total int64
	_ = s.svc.DB.WithContext(r.Context()).Model(&models.Notification{}).
		Where("user_id = ?", userID).Count(&total).Error

	var rows []models.Notification
	if err := s.svc.DB.WithContext(r.Context()).
		Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		s.writeError(w, r, apierr.Internal("Could not load notifications.", "تعذر تحميل الإشعارات."))
		return
	}
	var unread int64
	_ = s.svc.DB.WithContext(r.Context()).Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).Count(&unread).Error

	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"notifications": rows,
		"total":         total,
		"unread":        unread,
	})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	userID := CurrentUserID(r.Context())
	now := s.svc.Now()
	res := s.svc.DB.WithContext(r.Context()).Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"is_read": true, "read_at": &now})
	if res.Error != nil {
		s.writeError(w, r, apierr.Internal("Could not update the notification.", "تعذر تحديث الإشعار."))
		return
	}
	if res.RowsAffected == 0 {
		s.writeError(w, r, apierr.ErrNotFoundResource("Notification"))
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"id": id, "is_read": true})
}

// ensure service import is used even when handlers shrink (documents use it).
var _ = service.DocTypeNationalID
