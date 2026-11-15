package http

import (
	"net/http"

	"mawarid/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Provider profile (SRS FR-PRV-01..03)
// ---------------------------------------------------------------------------

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	out, err := s.prov.GetProfile(r.Context(), CurrentUserID(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var in service.ProviderProfileInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.prov.UpdateProfile(r.Context(), CurrentUserID(r.Context()), in)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Provider working hours (SRS FR-PRV-04)
// ---------------------------------------------------------------------------

func (s *Server) handleListWorkingHours(w http.ResponseWriter, r *http.Request) {
	rows, err := s.prov.ListWorkingHours(r.Context(), CurrentUserID(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"working_hours": rows})
}

// handleSetWorkingHours replaces the whole weekly schedule
// (PUT semantics — the SRS treats the schedule as one form).
func (s *Server) handleSetWorkingHours(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WorkingHours []service.WorkingHourInput `json:"working_hours"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	rows, err := s.prov.SetWorkingHours(r.Context(), CurrentUserID(r.Context()), in.WorkingHours)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"working_hours": rows})
}

// ---------------------------------------------------------------------------
// Provider locations (SRS FR-PRV-03 / Sprint 1 provider locations)
// ---------------------------------------------------------------------------

func (s *Server) handleListLocations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.prov.ListLocations(r.Context(), CurrentUserID(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"locations": rows})
}

func (s *Server) handleCreateLocation(w http.ResponseWriter, r *http.Request) {
	var in service.LocationInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.prov.CreateLocation(r.Context(), CurrentUserID(r.Context()), in)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, out)
}

// ---------------------------------------------------------------------------
// Provider logo (ERD Provider.logo_url)
// ---------------------------------------------------------------------------

// handleUploadLogo accepts multipart/form-data with a "file" field (JPG/PNG).
func (s *Server) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	file, header, _, err := parseUpload(w, r, s.svc.Cfg.Upload.MaxSizeBytes, false)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	defer file.Close() //nolint:errcheck

	logoURL, err := s.prov.UploadLogo(r.Context(), CurrentUserID(r.Context()),
		header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"logo_url":   logoURL,
		"message":    "Logo uploaded.",
		"message_ar": "تم رفع الشعار.",
	})
}

// handleServeLogo serves the stored logo image. It exposes only the commercial
// logo (never identity documents) with sniffed-safe content types.
func (s *Server) handleServeLogo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	abs, _, err := s.prov.LogoFilePath(r.Context(), id)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if id := RequestID(r.Context()); id != "" {
		w.Header().Set("X-Request-Id", id)
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, abs) //nolint:errcheck // ServeFile writes its own errors
}
