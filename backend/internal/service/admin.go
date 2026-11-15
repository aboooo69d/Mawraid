package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
)

// ---------------------------------------------------------------------------
// AdminService — account verification workflow (SRS URS 1.2 / FR-AUTH-12,
// NFR-AUTH-05 approve·reject, NFR-AUTH-06 suspend).
//
// Every privileged change writes an audit_logs row (actor + target) and, on
// account decisions, dispatches a best-effort SMS notification. Audit details
// never contain sensitive identity data (NFR-SEC-02).
// ---------------------------------------------------------------------------

// AdminService groups the back-office operations.
type AdminService struct {
	svc *Service
}

// NewAdminService builds the admin service.
func NewAdminService(svc *Service) *AdminService { return &AdminService{svc: svc} }

// ReviewDecision is the admin verdict on an account or document.
type ReviewDecision struct {
	Decision string `json:"decision"` // "approve" | "reject"
	Comment  string `json:"comment"`  // optional, stored as admin note
}

func (d ReviewDecision) validate() error {
	switch d.Decision {
	case "approve", "reject":
		return nil
	}
	return apierr.ErrValidation([]apierr.FieldViolation{{
		Field:   "decision",
		Message: `Must be "approve" or "reject". / يجب أن يكون approve أو reject.`,
	}})
}

func (d ReviewDecision) approved() bool { return d.Decision == "approve" }

// ---------------------------------------------------------------------------
// List accounts
// ---------------------------------------------------------------------------

// AdminListQuery filters the account list.
type AdminListQuery struct {
	Role   string // consumer | provider | "" (all)
	Status string // active | pending_approval | rejected | suspended | "" (all)
	Search string // matches name or phone
	Limit  int
	Offset int
}

// AdminAccountRow is the public admin view of an account (never includes
// password hashes, id numbers or document contents).
type AdminAccountRow struct {
	UserID             uint                      `json:"user_id"`
	Role               string                    `json:"role"`
	FullName           string                    `json:"full_name"`
	PhoneNumber        string                    `json:"phone_number"`
	Status             models.AccountStatus      `json:"status"`
	VerificationStatus models.VerificationStatus `json:"verification_status"`
	PhoneVerified      bool                      `json:"phone_verified"`
	ProviderName       string                    `json:"provider_name,omitempty"`
	PendingDocuments   int                       `json:"pending_documents"`
	AdminNote          *string                   `json:"admin_note,omitempty"`
	CreatedAt          time.Time                 `json:"created_at"`
}

// ListAccounts returns a filtered, paginated account list plus the total count.
func (a *AdminService) ListAccounts(ctx context.Context, q AdminListQuery) ([]AdminAccountRow, int64, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	tx := a.svc.DB.WithContext(ctx).Model(&models.User{}).Preload("Role")

	if q.Role != "" {
		switch q.Role {
		case models.RoleConsumer, models.RoleProvider, models.RoleDriver, models.RoleAdmin:
			tx = tx.Where("role_id = (SELECT id FROM roles WHERE name = ?)", q.Role)
		default:
			return nil, 0, apierr.ErrValidation([]apierr.FieldViolation{{
				Field: "role", Message: "Unknown role. / دور غير معروف.",
			}})
		}
	}
	if q.Status != "" {
		if !models.AccountStatus(q.Status).Valid() {
			return nil, 0, apierr.ErrValidation([]apierr.FieldViolation{{
				Field: "status", Message: "Unknown account status. / حالة حساب غير معروفة.",
			}})
		}
		tx = tx.Where("status = ?", q.Status)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		tx = tx.Where(
			a.svc.DB.WithContext(ctx).
				Where("LOWER(full_name) LIKE ?", like).
				Or("phone_number LIKE ?", "%"+s+"%"),
		)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load accounts.", "تعذر تحميل الحسابات.")
	}

	var users []models.User
	if err := tx.Order("created_at DESC").Limit(q.Limit).Offset(q.Offset).Find(&users).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load accounts.", "تعذر تحميل الحسابات.")
	}

	rows, err := a.decorate(ctx, users)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetAccount returns one account with its documents and provider profile.
func (a *AdminService) GetAccount(ctx context.Context, targetID uint) (map[string]interface{}, error) {
	var user models.User
	if err := a.svc.DB.WithContext(ctx).Preload("Role").First(&user, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierr.ErrNotFoundResource("Account")
		}
		return nil, apierr.Internal("Could not load the account.", "تعذر تحميل الحساب.")
	}
	rows, err := a.decorate(ctx, []models.User{user})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	docs, err := a.svc.ListDocuments(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{
		"account":   rows[0],
		"documents": docs,
	}
	if user.Role.Name == models.RoleProvider {
		var prov models.Provider
		if e := a.svc.DB.WithContext(ctx).First(&prov, user.ID).Error; e == nil {
			out["provider"] = prov
		}
	}
	return out, nil
}

// decorate adds role-specific and document counters to a user slice.
func (a *AdminService) decorate(ctx context.Context, users []models.User) ([]AdminAccountRow, error) {
	if len(users) == 0 {
		return []AdminAccountRow{}, nil
	}
	ids := make([]uint, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	// Pending document counts in one query (no N+1).
	type cnt struct {
		UserID uint
		N      int
	}
	var counts []cnt
	_ = a.svc.DB.WithContext(ctx).Model(&models.IdentityDocument{}).
		Select("user_id, COUNT(*) AS n").
		Where("user_id IN ? AND status = ?", ids, "pending").
		Group("user_id").Scan(&counts).Error
	pending := map[uint]int{}
	for _, c := range counts {
		pending[c.UserID] = c.N
	}

	// Provider names in one query.
	type pname struct {
		UserID uint
		Name   string
	}
	var pnames []pname
	_ = a.svc.DB.WithContext(ctx).Model(&models.Provider{}).
		Select("user_id, provider_name").
		Where("user_id IN ?", ids).Scan(&pnames).Error
	provNames := map[uint]string{}
	for _, p := range pnames {
		provNames[p.UserID] = p.Name
	}

	rows := make([]AdminAccountRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, AdminAccountRow{
			UserID: u.ID, Role: u.Role.Name, FullName: u.FullName,
			PhoneNumber: u.PhoneNumber, Status: u.Status,
			VerificationStatus: u.VerificationStatus,
			PhoneVerified:      u.PhoneVerifiedAt != nil,
			ProviderName:       provNames[u.ID],
			PendingDocuments:   pending[u.ID],
			AdminNote:          u.AdminNote,
			CreatedAt:          u.CreatedAt,
		})
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// Approve / reject (NFR-AUTH-05)
// ---------------------------------------------------------------------------

// ReviewAccount applies the admin verdict to an account:
//   - approve: status=active, pending documents approved, sessions kept;
//   - reject:  status=rejected, pending documents rejected, sessions revoked.
//
// The decision is written to the audit log and the user is notified by SMS
// (best effort — the SMS layer never fails the review itself).
func (a *AdminService) ReviewAccount(ctx context.Context, actorID, targetID uint, dec ReviewDecision) (AdminAccountRow, error) {
	if err := dec.validate(); err != nil {
		return AdminAccountRow{}, err
	}
	if actorID == targetID {
		return AdminAccountRow{}, apierr.Forbidden("SELF_REVIEW_FORBIDDEN",
			"You cannot review your own account.",
			"لا يمكنك مراجعة حسابك بنفسك.")
	}

	var user models.User
	if err := a.svc.DB.WithContext(ctx).Preload("Role").First(&user, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminAccountRow{}, apierr.ErrNotFoundResource("Account")
		}
		return AdminAccountRow{}, apierr.Internal("Could not load the account.", "تعذر تحميل الحساب.")
	}
	if user.Role.Name == models.RoleAdmin {
		return AdminAccountRow{}, apierr.Forbidden("ADMIN_TARGET_FORBIDDEN",
			"Administrator accounts cannot be reviewed here.",
			"لا يمكن مراجعة حسابات المسؤولين من هنا.")
	}

	now := time.Now().UTC()
	docStatus := "approved"
	if !dec.approved() {
		docStatus = "rejected"
	}
	newStatus := models.AccountStatusActive
	newVerif := user.VerificationStatus
	if !dec.approved() {
		newStatus = models.AccountStatusRejected
		if newVerif != models.VerificationUnverified {
			newVerif = models.VerificationRejected
		}
	} else if newVerif == models.VerificationPendingReview {
		newVerif = models.VerificationVerified
	}

	note := truncate(strings.TrimSpace(dec.Comment), 1000)

	txErr := a.svc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", targetID).Updates(map[string]any{
			"status":              newStatus,
			"verification_status": newVerif,
			"admin_note":          nullIfEmpty(note),
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		// Resolve every document still waiting for review.
		if err := tx.Model(&models.IdentityDocument{}).
			Where("user_id = ? AND status = ?", targetID, "pending").
			Updates(map[string]any{
				"status": docStatus, "reviewer_id": actorID,
				"review_note": nullIfEmpty(note), "reviewed_at": now, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		if !dec.approved() {
			// A rejected/suspended decision must close every live session (NFR-AUTH-06).
			if err := tx.Model(&models.AuthSession{}).
				Where("user_id = ? AND revoked_at IS NULL", targetID).
				Update("revoked_at", now).Error; err != nil {
				return err
			}
		}
		details := "decision=" + dec.Decision
		return tx.Create(&models.AuditLog{
			ActorUserID: &actorID, TargetUserID: &targetID,
			Action: "account.review." + dec.Decision, Details: &details,
		}).Error
	})
	if txErr != nil {
		return AdminAccountRow{}, apierr.Internal("Could not apply the review decision.", "تعذر تطبيق قرار المراجعة.")
	}

	a.notify(ctx, &user, dec)

	rows, err := a.decorate(ctx, []models.User{user})
	if err != nil || len(rows) == 0 {
		return AdminAccountRow{}, err
	}
	row := rows[0]
	row.Status = newStatus
	row.VerificationStatus = newVerif
	row.AdminNote = nullIfEmpty(note)
	return row, nil
}

// notify sends the decision SMS (best effort; never blocks or fails review).
func (a *AdminService) notify(ctx context.Context, u *models.User, dec ReviewDecision) {
	var msg string
	if dec.approved() {
		msg = "موارد: تم اعتماد حسابك بنجاح، يمكنك الآن تسجيل الدخول."
	} else {
		msg = "موارد: نأسف، تم رفض طلب حسابك. للتفاصيل تواصل مع الدعم."
	}
	if err := a.svc.SMS.Send(ctx, u.PhoneNumber, msg); err != nil {
		a.svc.Log.WarnContext(ctx, "decision sms failed", "err", err, "user_id", u.ID)
	}
}

// ---------------------------------------------------------------------------
// Suspend / reinstate (NFR-AUTH-06)
// ---------------------------------------------------------------------------

// Suspend blocks the account (login and refresh are rejected) and revokes its
// sessions. reason is stored as the admin note and in the audit log.
func (a *AdminService) Suspend(ctx context.Context, actorID, targetID uint, reason string) (AdminAccountRow, error) {
	return a.transition(ctx, actorID, targetID, models.AccountStatusSuspended, "suspend", reason)
}

// Reinstate reactivates a suspended or rejected account.
func (a *AdminService) Reinstate(ctx context.Context, actorID, targetID uint, reason string) (AdminAccountRow, error) {
	return a.transition(ctx, actorID, targetID, models.AccountStatusActive, "reinstate", reason)
}

func (a *AdminService) transition(ctx context.Context, actorID, targetID uint, status models.AccountStatus, action, reason string) (AdminAccountRow, error) {
	if actorID == targetID {
		return AdminAccountRow{}, apierr.Forbidden("SELF_REVIEW_FORBIDDEN",
			"You cannot change the status of your own account.",
			"لا يمكنك تغيير حالة حسابك بنفسك.")
	}
	var user models.User
	if err := a.svc.DB.WithContext(ctx).Preload("Role").First(&user, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminAccountRow{}, apierr.ErrNotFoundResource("Account")
		}
		return AdminAccountRow{}, apierr.Internal("Could not load the account.", "تعذر تحميل الحساب.")
	}
	if user.Role.Name == models.RoleAdmin {
		return AdminAccountRow{}, apierr.Forbidden("ADMIN_TARGET_FORBIDDEN",
			"Administrator accounts cannot be changed here.",
			"لا يمكن تغيير حسابات المسؤولين من هنا.")
	}

	now := time.Now().UTC()
	note := truncate(strings.TrimSpace(reason), 1000)
	details := "status=" + string(status)

	txErr := a.svc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", targetID).Updates(map[string]any{
			"status": status, "admin_note": nullIfEmpty(note), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if status != models.AccountStatusActive {
			if err := tx.Model(&models.AuthSession{}).
				Where("user_id = ? AND revoked_at IS NULL", targetID).
				Update("revoked_at", now).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.AuditLog{
			ActorUserID: &actorID, TargetUserID: &targetID,
			Action: "account." + action, Details: &details,
		}).Error
	})
	if txErr != nil {
		return AdminAccountRow{}, apierr.Internal("Could not change the account status.", "تعذر تغيير حالة الحساب.")
	}

	// Best-effort SMS notification (NFR-AUTH-11).
	var smsMsg string
	switch status {
	case models.AccountStatusSuspended:
		smsMsg = "موارد: تم إيقاف حسابك مؤقتاً. للتفاصيل تواصل مع الدعم."
	case models.AccountStatusActive:
		smsMsg = "موارد: تم إعادة تفعيل حسابك، يمكنك الآن تسجيل الدخول."
	}
	if smsMsg != "" {
		if err := a.svc.SMS.Send(ctx, user.PhoneNumber, smsMsg); err != nil {
			a.svc.Log.WarnContext(ctx, "status sms failed", "err", err, "user_id", targetID)
		}
	}

	rows, err := a.decorate(ctx, []models.User{user})
	if err != nil || len(rows) == 0 {
		return AdminAccountRow{}, err
	}
	row := rows[0]
	row.Status = status
	row.AdminNote = nullIfEmpty(note)
	return row, nil
}

// ---------------------------------------------------------------------------
// Document review (SRS FR-CON-05 → admin validates uploaded documents)
// ---------------------------------------------------------------------------

// ReviewDocument validates a single uploaded document. When no other document
// of that user is pending, the user's verification_status is recomputed
// (verified / rejected).
func (a *AdminService) ReviewDocument(ctx context.Context, actorID, docID uint, dec ReviewDecision) (DocumentMeta, error) {
	if err := dec.validate(); err != nil {
		return DocumentMeta{}, err
	}
	var doc models.IdentityDocument
	if err := a.svc.DB.WithContext(ctx).First(&doc, docID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentMeta{}, apierr.ErrNotFoundResource("Document")
		}
		return DocumentMeta{}, apierr.Internal("Could not load the document.", "تعذر تحميل الوثيقة.")
	}
	if doc.Status != "pending" {
		return DocumentMeta{}, apierr.Conflict("DOCUMENT_ALREADY_REVIEWED",
			"This document has already been reviewed.",
			"تمت مراجعة هذه الوثيقة مسبقاً.")
	}

	now := time.Now().UTC()
	docStatus := "approved"
	if !dec.approved() {
		docStatus = "rejected"
	}
	note := truncate(strings.TrimSpace(dec.Comment), 1000)

	txErr := a.svc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.IdentityDocument{}).Where("id = ?", docID).Updates(map[string]any{
			"status": docStatus, "reviewer_id": actorID,
			"review_note": nullIfEmpty(note), "reviewed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		details := "document_id=" + strconv.FormatUint(uint64(docID), 10) + ";decision=" + dec.Decision
		return tx.Create(&models.AuditLog{
			ActorUserID: &actorID, TargetUserID: &doc.UserID,
			Action: "document.review." + dec.Decision, Details: &details,
		}).Error
	})
	if txErr != nil {
		return DocumentMeta{}, apierr.Internal("Could not review the document.", "تعذر مراجعة الوثيقة.")
	}

	// Recompute the user's verification status from the remaining documents.
	a.recomputeVerification(ctx, doc.UserID)

	// Reload for the response.
	_ = a.svc.DB.WithContext(ctx).First(&doc, docID).Error
	return toDocumentMeta(&doc), nil
}

// recomputeVerification sets users.verification_status from the document set:
// pending > approved > (rejected | none).
func (a *AdminService) recomputeVerification(ctx context.Context, userID uint) {
	type sc struct {
		Status string
		N      int64
	}
	var stats []sc
	_ = a.svc.DB.WithContext(ctx).Model(&models.IdentityDocument{}).
		Select("status, COUNT(*) AS n").
		Where("user_id = ?", userID).
		Group("status").Scan(&stats).Error

	var pending, approved, total int64
	for _, s := range stats {
		total += s.N
		switch s.Status {
		case "pending":
			pending = s.N
		case "approved":
			approved = s.N
		}
	}

	var next models.VerificationStatus
	switch {
	case pending > 0:
		next = models.VerificationPendingReview
	case total == 0:
		next = models.VerificationUnverified
	case approved > 0:
		next = models.VerificationVerified
	default:
		next = models.VerificationRejected
	}
	// Never downgrade an account that has not entered the workflow.
	if total == 0 {
		return
	}
	_ = a.svc.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]any{"verification_status": next, "updated_at": time.Now().UTC()}).Error
}

// ---------------------------------------------------------------------------
// Pending verification queue
// ---------------------------------------------------------------------------

// PendingVerification is one account waiting for an admin decision.
type PendingVerification struct {
	UserID             uint                      `json:"user_id"`
	Role               string                    `json:"role"`
	FullName           string                    `json:"full_name"`
	PhoneNumber        string                    `json:"phone_number"`
	Status             models.AccountStatus      `json:"status"`
	VerificationStatus models.VerificationStatus `json:"verification_status"`
	ProviderName       string                    `json:"provider_name,omitempty"`
	SubmittedAt        time.Time                 `json:"created_at"`
	Documents          []DocumentMeta            `json:"documents"`
}

// ListPendingVerifications returns accounts that still need a decision
// (status=pending_approval, or verification_status=pending_review, or with
// documents waiting for review).
func (a *AdminService) ListPendingVerifications(ctx context.Context, limit, offset int) ([]PendingVerification, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	tx := a.svc.DB.WithContext(ctx).Model(&models.User{}).Preload("Role").
		Where("users.status = ? OR users.verification_status = ? OR EXISTS (SELECT 1 FROM identity_documents d WHERE d.user_id = users.id AND d.status = 'pending')",
			models.AccountStatusPendingApproval, models.VerificationPendingReview)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load the queue.", "تعذر تحميل قائمة المراجعة.")
	}

	var users []models.User
	if err := tx.Order("users.created_at ASC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load the queue.", "تعذر تحميل قائمة المراجعة.")
	}
	if len(users) == 0 {
		return []PendingVerification{}, total, nil
	}

	ids := make([]uint, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	var docs []models.IdentityDocument
	_ = a.svc.DB.WithContext(ctx).Where("user_id IN ?", ids).Order("created_at ASC").Find(&docs).Error
	docsByUser := map[uint][]DocumentMeta{}
	for i := range docs {
		docsByUser[docs[i].UserID] = append(docsByUser[docs[i].UserID], toDocumentMeta(&docs[i]))
	}
	var pnames []struct {
		UserID uint
		Name   string
	}
	_ = a.svc.DB.WithContext(ctx).Model(&models.Provider{}).
		Select("user_id, provider_name").Where("user_id IN ?", ids).Scan(&pnames).Error
	provNames := map[uint]string{}
	for _, p := range pnames {
		provNames[p.UserID] = p.Name
	}

	out := make([]PendingVerification, 0, len(users))
	for _, u := range users {
		out = append(out, PendingVerification{
			UserID: u.ID, Role: u.Role.Name, FullName: u.FullName,
			PhoneNumber: u.PhoneNumber, Status: u.Status,
			VerificationStatus: u.VerificationStatus,
			ProviderName:       provNames[u.ID],
			SubmittedAt:        u.CreatedAt,
			Documents:          docsByUser[u.ID],
		})
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// Audit trail (admin read-only)
// ---------------------------------------------------------------------------

// AuditEntry is the public view of an audit_logs row.
type AuditEntry struct {
	ID           uint      `json:"id"`
	ActorUserID  *uint     `json:"actor_user_id,omitempty"`
	TargetUserID *uint     `json:"target_user_id,omitempty"`
	Action       string    `json:"action"`
	Details      *string   `json:"details,omitempty"`
	IP           *string   `json:"ip,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListAuditLogs returns the most recent audit entries (admin only).
func (a *AdminService) ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditEntry, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	tx := a.svc.DB.WithContext(ctx).Model(&models.AuditLog{})
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load the audit log.", "تعذر تحميل سجل التدقيق.")
	}
	var rows []models.AuditLog
	if err := tx.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load the audit log.", "تعذر تحميل سجل التدقيق.")
	}
	out := make([]AuditEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, AuditEntry{
			ID: r.ID, ActorUserID: r.ActorUserID, TargetUserID: r.TargetUserID,
			Action: r.Action, Details: r.Details, IP: r.IP, CreatedAt: r.CreatedAt,
		})
	}
	return out, total, nil
}

// --- small helpers ------------------------------------------------------------

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
