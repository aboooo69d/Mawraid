package http

import (
	"net/http"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
)

// routes registers every endpoint of the Sprint 1 API (see api/openapi.yaml).
//
// Public          — registration, OTP, login/refresh/logout, password recovery,
//
//	health, provider logo.
//
// Any role        — current user, account status, verification, documents,
//
//	notifications.
//
// provider only   — profile, working hours, locations, logo upload.
// admin only      — accounts, review/approve/reject/suspend, documents,
//
//	verification queue, audit log, provider listings.
func (s *Server) routes() {
	base := s.cfg.HTTP.APIBasePath // default /api/v1

	// --- health (outside the versioned base for infra probes) --------------
	s.mux.Handle("GET /health", s.wrap(http.HandlerFunc(s.handleHealth)))
	s.mux.Handle("GET /healthz", s.wrap(http.HandlerFunc(s.handleHealth)))

	// --- public auth --------------------------------------------------------
	s.pub("POST "+base+"/auth/register/consumer", s.handleRegisterConsumer)
	s.pub("POST "+base+"/auth/register/provider", s.handleRegisterProvider)
	s.pub("POST "+base+"/auth/otp/request", s.handleOTPRequest)
	s.pub("POST "+base+"/auth/otp/verify", s.handleOTPVerify)
	s.pub("POST "+base+"/auth/login", s.handleLogin)
	s.pub("POST "+base+"/auth/refresh", s.handleRefresh)
	s.pub("POST "+base+"/auth/forgot-password", s.handleForgotPassword)
	s.pub("POST "+base+"/auth/reset-password", s.handleResetPassword)

	// --- authenticated (any verified role) ----------------------------------
	s.priv("POST "+base+"/auth/logout", s.handleLogout)
	s.priv("GET "+base+"/auth/me", s.handleMe)

	s.priv("GET "+base+"/account/status", s.handleAccountStatus)
	s.priv("POST "+base+"/account/verification", s.handleSubmitVerification)
	s.priv("GET "+base+"/account/documents", s.handleListDocuments)
	s.priv("POST "+base+"/account/documents", s.handleUploadDocument)
	s.priv("GET "+base+"/account/documents/{id}/download", s.handleDownloadDocument)

	s.priv("GET "+base+"/notifications", s.handleListNotifications)
	s.priv("PATCH "+base+"/notifications/{id}/read", s.handleMarkNotificationRead)

	// --- provider (role: provider) -----------------------------------------
	s.only(models.RoleProvider,
		"GET "+base+"/providers/me/profile",
		s.handleGetProfile)
	s.only(models.RoleProvider,
		"PUT "+base+"/providers/me/profile",
		s.handleUpdateProfile)
	s.only(models.RoleProvider,
		"GET "+base+"/providers/me/working-hours",
		s.handleListWorkingHours)
	s.only(models.RoleProvider,
		"PUT "+base+"/providers/me/working-hours",
		s.handleSetWorkingHours)
	s.only(models.RoleProvider,
		"GET "+base+"/providers/me/locations",
		s.handleListLocations)
	s.only(models.RoleProvider,
		"POST "+base+"/providers/me/locations",
		s.handleCreateLocation)
	s.only(models.RoleProvider,
		"POST "+base+"/providers/me/logo",
		s.handleUploadLogo)

	// Public logo serving (non-sensitive commercial image).
	s.pub("GET "+base+"/providers/{id}/logo", s.handleServeLogo)

	// --- admin (role: admin) ------------------------------------------------
	s.only(models.RoleAdmin, "GET "+base+"/admin/accounts", s.handleAdminListAccounts)
	s.only(models.RoleAdmin, "GET "+base+"/admin/accounts/{id}", s.handleAdminGetAccount)
	s.only(models.RoleAdmin, "POST "+base+"/admin/accounts/{id}/review", s.handleAdminReviewAccount)
	s.only(models.RoleAdmin, "POST "+base+"/admin/accounts/{id}/suspend", s.handleAdminSuspend)
	s.only(models.RoleAdmin, "POST "+base+"/admin/accounts/{id}/reinstate", s.handleAdminReinstate)
	s.only(models.RoleAdmin, "POST "+base+"/admin/accounts/{id}/documents", s.handleAdminUploadDocument)
	s.only(models.RoleAdmin, "GET "+base+"/admin/verifications", s.handleAdminListVerifications)
	s.only(models.RoleAdmin, "POST "+base+"/admin/documents/{id}/review", s.handleAdminReviewDocument)
	s.only(models.RoleAdmin, "GET "+base+"/admin/audit-logs", s.handleAdminAuditLogs)
	s.only(models.RoleAdmin, "GET "+base+"/admin/providers", s.handleAdminListProviders)
	s.only(models.RoleAdmin, "GET "+base+"/admin/providers/stats", s.handleAdminProviderStats)

	// --- catch-all under the API base → JSON 404 envelope -------------------
	s.mux.Handle(base+"/", s.wrap(http.HandlerFunc(s.handleNotFound)))
}

// pub registers a public route.
func (s *Server) pub(pattern string, h http.HandlerFunc) {
	s.mux.Handle(pattern, s.wrap(h))
}

// priv registers an authenticated route (any active role).
func (s *Server) priv(pattern string, h http.HandlerFunc) {
	s.mux.Handle(pattern, s.wrap(s.authenticate(h)))
}

// only registers an authenticated route restricted to the given roles.
func (s *Server) only(role string, pattern string, h http.HandlerFunc) {
	s.mux.Handle(pattern, s.wrap(s.authenticate(s.requireRole(role)(h))))
}

// handleNotFound is the JSON 404 fallback below the API base.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, apierr.NotFound("ENDPOINT_NOT_FOUND",
		"The requested endpoint does not exist.",
		"النقطة المطلوبة غير موجودة."))
}

// handleHealth reports process liveness and component reachability.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	db := "ok"
	if err := s.svc.DB.WithContext(r.Context()).Raw("SELECT 1").Error; err != nil {
		db = "unavailable"
	}
	redis := "disabled"
	if !s.cfg.Redis.Disabled && s.svc.Limiter != nil {
		if _, _, err := s.svc.Limiter.Peek(r.Context(), "health"); err != nil {
			redis = "unavailable"
		} else {
			redis = "ok"
		}
	}
	status := http.StatusOK
	if db != "ok" {
		status = http.StatusServiceUnavailable
	}
	s.writeJSON(w, r, status, map[string]any{
		"status": map[bool]string{true: "ok", false: "degraded"}[db == "ok"],
		"db":     db,
		"redis":  redis,
		"sms": map[string]any{
			"adapter":       s.svc.SMS.Name(),
			"real_delivery": s.svc.SMS.RealDelivery(),
		},
	})
}
