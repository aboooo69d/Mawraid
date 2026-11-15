package http

import (
	"net/http"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Admin — account verification workflow (SRS URS 1.2, FR-AUTH-12,
// NFR-AUTH-05 approve·reject, NFR-AUTH-06 suspend).
// ---------------------------------------------------------------------------

func (s *Server) handleAdminListAccounts(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	rows, total, err := s.admin.ListAccounts(r.Context(), service.AdminListQuery{
		Role:   r.URL.Query().Get("role"),
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"accounts": rows, "total": total, "limit": limit, "offset": offset,
	})
}

func (s *Server) handleAdminGetAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.admin.GetAccount(r.Context(), id)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// handleAdminReviewAccount applies the NFR-AUTH-05 verdict:
// POST /admin/accounts/{id}/review {"decision":"approve"|"reject","comment":""}
func (s *Server) handleAdminReviewAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var in service.ReviewDecision
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	row, err := s.admin.ReviewAccount(r.Context(), CurrentUserID(r.Context()), id, in)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"account":    row,
		"message":    "Decision applied.",
		"message_ar": "تم تطبيق القرار.",
	})
}

type reasonInput struct {
	Reason string `json:"reason"`
}

func (s *Server) handleAdminSuspend(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var in reasonInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	row, err := s.admin.Suspend(r.Context(), CurrentUserID(r.Context()), id, in.Reason)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"account":    row,
		"message":    "Account suspended; active sessions revoked.",
		"message_ar": "تم إيقاف الحساب وإلغاء الجلسات النشطة.",
	})
}

func (s *Server) handleAdminReinstate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var in reasonInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	row, err := s.admin.Reinstate(r.Context(), CurrentUserID(r.Context()), id, in.Reason)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"account":    row,
		"message":    "Account reactivated.",
		"message_ar": "تم إعادة تفعيل الحساب.",
	})
}

// handleAdminListVerifications returns the pending review queue.
func (s *Server) handleAdminListVerifications(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	rows, total, err := s.admin.ListPendingVerifications(r.Context(), limit, offset)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"verifications": rows, "total": total, "limit": limit, "offset": offset,
	})
}

// handleAdminReviewDocument validates one uploaded document
// (FR-CON-05: admin validates before the account can be approved).
func (s *Server) handleAdminReviewDocument(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var in service.ReviewDecision
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	doc, err := s.admin.ReviewDocument(r.Context(), CurrentUserID(r.Context()), id, in)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"document":   doc,
		"message":    "Document decision recorded.",
		"message_ar": "تم تسجيل قرار الوثيقة.",
	})
}

// handleAdminAuditLogs exposes the audit trail (admin only, NFR-SEC-02).
func (s *Server) handleAdminAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	rows, total, err := s.admin.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"logs": rows, "total": total, "limit": limit, "offset": offset,
	})
}

// ---------------------------------------------------------------------------
// Admin — provider listings
// ---------------------------------------------------------------------------

func (s *Server) handleAdminListProviders(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	rows, total, err := s.prov.AdminListProviders(r.Context(),
		r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"providers": rows, "total": total, "limit": limit, "offset": offset,
	})
}

func (s *Server) handleAdminProviderStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.prov.ProviderStats(r.Context())
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, stats)
}

// compile-time guard: apierr is used by the shared helpers in this package.
var _ = apierr.Internal
