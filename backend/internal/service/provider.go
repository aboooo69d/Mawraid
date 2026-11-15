package service

import (
	"context"
	"errors"
	"fmt"
	"io"
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
// ProviderService — provider profile, locations and working hours
// (SRS FR-PRV-01..04, ERD Provider / provider_locations /
// provider_working_hours ADDITION tables).
//
// Note: models.Provider's primary key IS the user id (ERD 1:1 link), so every
// provider-scoped row is keyed by user_id. provider_locations.provider_id
// also references that same user id (kept ERD-compatible).
// ---------------------------------------------------------------------------

// ProviderService groups provider-facing operations.
type ProviderService struct {
	svc *Service
}

// NewProviderService builds the provider service.
func NewProviderService(svc *Service) *ProviderService { return &ProviderService{svc: svc} }

// getProvider loads the provider profile row for a user (404 when missing).
func (p *ProviderService) getProvider(ctx context.Context, userID uint) (*models.Provider, error) {
	var prov models.Provider
	if err := p.svc.DB.WithContext(ctx).First(&prov, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierr.ErrNotFoundResource("Provider profile")
		}
		return nil, apierr.Internal("Could not load the provider profile.", "تعذر تحميل ملف المورد.")
	}
	return &prov, nil
}

// requireProviderRole rejects callers that are not providers.
func (p *ProviderService) requireRole(ctx context.Context, userID uint, role string) error {
	var user models.User
	if err := p.svc.DB.WithContext(ctx).Preload("Role").First(&user, userID).Error; err != nil {
		return apierr.ErrNotFoundResource("Account")
	}
	if user.Role.Name != role {
		return apierr.ErrForbiddenRole()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Profile (FR-PRV-01..03)
// ---------------------------------------------------------------------------

// ProviderProfileInput — PUT/PATCH body of /providers/me/profile.
type ProviderProfileInput struct {
	ProviderName        string   `json:"provider_name"`
	ProviderDescription string   `json:"provider_description"`
	StartTime           string   `json:"start_time"` // "HH:MM"
	EndTime             string   `json:"end_time"`   // "HH:MM", must be > start_time
	City                string   `json:"city"`
	District            string   `json:"district"`
	Street              string   `json:"street"`
	AdditionalInfo      string   `json:"additional_info"`
	Latitude            *float64 `json:"latitude"`
	Longitude           *float64 `json:"longitude"`
}

// ProfileResponse is the full provider profile view.
type ProfileResponse struct {
	Provider     map[string]interface{} `json:"provider"`
	Locations    []LocationView         `json:"locations"`
	WorkingHours []WorkingHourView      `json:"working_hours"`
}

// GetProfile returns the provider profile with its locations and hours.
func (p *ProviderService) GetProfile(ctx context.Context, userID uint) (ProfileResponse, error) {
	prov, err := p.getProvider(ctx, userID)
	if err != nil {
		return ProfileResponse{}, err
	}
	locs, err := p.ListLocations(ctx, userID)
	if err != nil {
		return ProfileResponse{}, err
	}
	hours, err := p.ListWorkingHours(ctx, userID)
	if err != nil {
		return ProfileResponse{}, err
	}
	return ProfileResponse{Provider: providerView(prov), Locations: locs, WorkingHours: hours}, nil
}

func providerView(prov *models.Provider) map[string]interface{} {
	return map[string]interface{}{
		"user_id":              prov.UserID,
		"provider_name":        prov.ProviderName,
		"provider_description": deref(prov.Description),
		"start_time":           deref(prov.StartTime),
		"end_time":             deref(prov.EndTime),
		"logo_url":             deref(prov.LogoURL),
		"created_at":           prov.CreatedAt,
		"updated_at":           prov.UpdatedAt,
	}
}

// UpdateProfile updates the provider core profile (FR-PRV-01/02/03) and, when
// location fields are present, upserts the provider's primary location.
func (p *ProviderService) UpdateProfile(ctx context.Context, userID uint, in ProviderProfileInput) (ProfileResponse, error) {
	var v validation.Violations

	name := validation.SanitizeText(in.ProviderName)
	v.Required("provider_name", name)
	v.TextLength("provider_name", name, 2, 120)

	desc := validation.SanitizeText(in.ProviderDescription)
	if desc != "" {
		v.TextLength("provider_description", desc, 2, 500) // FR-PRV-01 max 500
	}

	start, end := strings.TrimSpace(in.StartTime), strings.TrimSpace(in.EndTime)
	if start != "" || end != "" {
		v.Required("start_time", start)
		v.Required("end_time", end)
		v.TimeHHMM("start_time", start)
		v.TimeHHMM("end_time", end)
		if validTimeRe.MatchString(start) && validTimeRe.MatchString(end) && end <= start {
			// SRS 1.2 scenario 3 / FR-PRV-02.
			v.Add("end_time",
				"End time must be after start time.",
				"يجب أن يكون وقت انتهاء العمل بعد وقت بدء العمل.")
		}
	}

	if in.Latitude != nil && in.Longitude == nil || in.Latitude == nil && in.Longitude != nil {
		v.Add("latitude", "latitude and longitude must be provided together.",
			"يجب إدخال خط الطول وخط العرض معاً.")
	}
	if in.Latitude != nil && (*in.Latitude < -90 || *in.Latitude > 90) {
		v.Add("latitude", "latitude must be between -90 and 90.",
			"خط العرض يجب أن يكون بين -90 و 90.")
	}
	if in.Longitude != nil && (*in.Longitude < -180 || *in.Longitude > 180) {
		v.Add("longitude", "longitude must be between -180 and 180.",
			"خط الطول يجب أن يكون بين -180 و 180.")
	}

	if err := v.Err(); err != nil {
		return ProfileResponse{}, err
	}

	prov, err := p.getProvider(ctx, userID)
	if err != nil {
		return ProfileResponse{}, err
	}

	now := time.Now().UTC()
	prov.ProviderName = name
	if desc != "" {
		prov.Description = &desc
	}
	if start != "" {
		prov.StartTime = &start
		prov.EndTime = &end
	}
	prov.UpdatedAt = now

	locationTouched := in.City != "" || in.District != "" || in.Street != "" ||
		in.AdditionalInfo != "" || in.Latitude != nil

	txErr := p.svc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(prov).Error; err != nil {
			return err
		}
		if locationTouched {
			if err := upsertPrimaryLocation(tx, prov, in, now); err != nil {
				return err
			}
		}
		details := "provider profile updated"
		return tx.Create(&models.AuditLog{
			ActorUserID: &userID, TargetUserID: &userID,
			Action: "provider.profile.updated", Details: &details,
		}).Error
	})
	if txErr != nil {
		return ProfileResponse{}, apierr.Internal("Could not update the provider profile.", "تعذر تحديث ملف المورد.")
	}

	return p.GetProfile(ctx, userID)
}

// upsertPrimaryLocation updates the oldest active location of the provider or
// creates one when none exists (registration always creates the first one).
func upsertPrimaryLocation(tx *gorm.DB, prov *models.Provider, in ProviderProfileInput, now time.Time) error {
	var loc models.ProviderLocation
	err := tx.Where("provider_id = ? AND deleted_at IS NULL", prov.UserID).
		Order("id ASC").First(&loc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		city := validation.SanitizeText(in.City)
		if city == "" {
			// Creating a location requires a city (models constraint: not null).
			return apierr.ErrValidation([]apierr.FieldViolation{{
				Field: "city", Message: "City is required. / المدينة مطلوبة.",
			}})
		}
		loc = models.ProviderLocation{
			ProviderID:     prov.UserID,
			Name:           prov.ProviderName,
			City:           city,
			AdditionalInfo: strPtr(validation.SanitizeText(in.AdditionalInfo)),
			Latitude:       in.Latitude,
			Longitude:      in.Longitude,
			IsActive:       true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if d := validation.SanitizeText(in.District); d != "" {
			loc.District = &d
		}
		if s := validation.SanitizeText(in.Street); s != "" {
			loc.Street = &s
		}
		return tx.Create(&loc).Error
	}
	if err != nil {
		return err
	}

	if c := validation.SanitizeText(in.City); c != "" {
		loc.City = c
	}
	if d := validation.SanitizeText(in.District); d != "" {
		loc.District = &d
	}
	if s := validation.SanitizeText(in.Street); s != "" {
		loc.Street = &s
	}
	if a := validation.SanitizeText(in.AdditionalInfo); a != "" {
		loc.AdditionalInfo = &a
	}
	if in.Latitude != nil {
		loc.Latitude = in.Latitude
		loc.Longitude = in.Longitude
	}
	loc.UpdatedAt = now
	return tx.Save(&loc).Error
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------------------------------------------------------------------------
// Working hours (FR-PRV-04 / Sprint 1 "provider working hours")
// ---------------------------------------------------------------------------

// WorkingHourView is the public view of one weekly slot.
type WorkingHourView struct {
	ID         uint   `json:"hour_id"`
	LocationID *uint  `json:"location_id,omitempty"`
	DayOfWeek  int    `json:"day_of_week"` // 0=Sunday … 6=Saturday
	StartTime  string `json:"start_time"`  // "HH:MM"
	EndTime    string `json:"end_time"`    // "HH:MM"
	IsActive   bool   `json:"is_active"`
}

// WorkingHourInput is one entry of a submitted schedule.
type WorkingHourInput struct {
	LocationID *uint  `json:"location_id"`
	DayOfWeek  int    `json:"day_of_week"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	IsActive   *bool  `json:"is_active"` // default true
}

// ListWorkingHours returns the provider's weekly schedule.
func (p *ProviderService) ListWorkingHours(ctx context.Context, userID uint) ([]WorkingHourView, error) {
	var rows []models.ProviderWorkingHour
	if err := p.svc.DB.WithContext(ctx).
		Where("provider_id = ?", userID).
		Order("day_of_week ASC, start_time ASC").Find(&rows).Error; err != nil {
		return nil, apierr.Internal("Could not load working hours.", "تعذر تحميل ساعات العمل.")
	}
	out := make([]WorkingHourView, 0, len(rows))
	for _, r := range rows {
		out = append(out, WorkingHourView{
			ID: r.ID, LocationID: r.LocationID, DayOfWeek: r.DayOfWeek,
			StartTime: r.StartTime, EndTime: r.EndTime, IsActive: r.IsActive,
		})
	}
	return out, nil
}

// SetWorkingHours replaces the full weekly schedule of the provider
// (FR-PRV-04 + validation: HH:MM format, end after start, day 0–6, max 7).
func (p *ProviderService) SetWorkingHours(ctx context.Context, userID uint, in []WorkingHourInput) ([]WorkingHourView, error) {
	if len(in) == 0 {
		return nil, apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "working_hours", Message: "Provide at least one slot. / يرجى إدخال فترة عمل واحدة على الأقل.",
		}})
	}
	if len(in) > 7 {
		return nil, apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "working_hours", Message: "Maximum 7 slots (one per day). / الحد الأقصى 7 فترات (واحدة لكل يوم).",
		}})
	}

	var v validation.Violations
	seen := map[int]bool{}
	ownedLocations := map[uint]bool{}
	var locRows []models.ProviderLocation
	_ = p.svc.DB.WithContext(ctx).Where("provider_id = ? AND deleted_at IS NULL", userID).Find(&locRows).Error
	for _, l := range locRows {
		ownedLocations[l.ID] = true
	}

	for i, h := range in {
		field := fmt.Sprintf("working_hours[%d]", i)
		if h.DayOfWeek < 0 || h.DayOfWeek > 6 {
			v.Add(field+".day_of_week",
				"day_of_week must be between 0 (Sunday) and 6 (Saturday).",
				"يجب أن يكون يوم الأسبوع بين 0 (الأحد) و 6 (السبت).")
		} else if seen[h.DayOfWeek] {
			v.Add(field+".day_of_week",
				"Duplicate day_of_week entry.",
				"تكرار في يوم الأسبوع.")
		} else {
			seen[h.DayOfWeek] = true
		}

		start, end := strings.TrimSpace(h.StartTime), strings.TrimSpace(h.EndTime)
		v.TimeHHMM(field+".start_time", start)
		v.TimeHHMM(field+".end_time", end)
		if validTimeRe.MatchString(start) && validTimeRe.MatchString(end) && end <= start {
			v.Add(field+".end_time",
				"End time must be after start time.",
				"يجب أن يكون وقت الإغلاق بعد وقت الفتح.")
		}
		if h.LocationID != nil && !ownedLocations[*h.LocationID] {
			v.Add(field+".location_id",
				"Unknown location for this provider.",
				"موقع غير معروف لهذا المورد.")
		}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	txErr := p.svc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", userID).Delete(&models.ProviderWorkingHour{}).Error; err != nil {
			return err
		}
		for _, h := range in {
			active := true
			if h.IsActive != nil {
				active = *h.IsActive
			}
			row := models.ProviderWorkingHour{
				ProviderID: userID, LocationID: h.LocationID,
				DayOfWeek: h.DayOfWeek,
				StartTime: strings.TrimSpace(h.StartTime),
				EndTime:   strings.TrimSpace(h.EndTime),
				IsActive:  active,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		details := fmt.Sprintf("slots=%d", len(in))
		return tx.Create(&models.AuditLog{
			ActorUserID: &userID, TargetUserID: &userID,
			Action: "provider.hours.updated", Details: &details,
		}).Error
	})
	if txErr != nil {
		return nil, apierr.Internal("Could not save working hours.", "تعذر حفظ ساعات العمل.")
	}
	return p.ListWorkingHours(ctx, userID)
}

// ---------------------------------------------------------------------------
// Locations (FR-PRV-03 / Sprint 1 "provider locations")
// ---------------------------------------------------------------------------

// LocationView is the public view of a distribution point.
type LocationView struct {
	ID             uint      `json:"location_id"`
	Name           string    `json:"name"`
	City           string    `json:"city"`
	District       string    `json:"district,omitempty"`
	Street         string    `json:"street,omitempty"`
	AdditionalInfo string    `json:"additional_info,omitempty"`
	Latitude       *float64  `json:"latitude,omitempty"`
	Longitude      *float64  `json:"longitude,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// LocationInput is the create-location body.
type LocationInput struct {
	Name           string   `json:"name"`
	City           string   `json:"city"`
	District       string   `json:"district"`
	Street         string   `json:"street"`
	AdditionalInfo string   `json:"additional_info"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	IsActive       *bool    `json:"is_active"`
}

func locationView(l models.ProviderLocation) LocationView {
	return LocationView{
		ID: l.ID, Name: l.Name, City: l.City,
		District:       deref(l.District),
		Street:         deref(l.Street),
		AdditionalInfo: deref(l.AdditionalInfo),
		Latitude:       l.Latitude, Longitude: l.Longitude,
		IsActive: l.IsActive, CreatedAt: l.CreatedAt,
	}
}

// ListLocations returns the provider's distribution points.
func (p *ProviderService) ListLocations(ctx context.Context, userID uint) ([]LocationView, error) {
	var rows []models.ProviderLocation
	if err := p.svc.DB.WithContext(ctx).
		Where("provider_id = ? AND deleted_at IS NULL", userID).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, apierr.Internal("Could not load locations.", "تعذر تحميل المواقع.")
	}
	out := make([]LocationView, 0, len(rows))
	for _, r := range rows {
		out = append(out, locationView(r))
	}
	return out, nil
}

// CreateLocation adds a new distribution point (city required by the ERD).
func (p *ProviderService) CreateLocation(ctx context.Context, userID uint, in LocationInput) (LocationView, error) {
	var v validation.Violations
	name := validation.SanitizeText(in.Name)
	v.Required("name", name)
	v.TextLength("name", name, 2, 120)
	city := validation.SanitizeText(in.City)
	v.Required("city", city)
	v.TextLength("city", city, 2, 80)
	if in.Latitude != nil && in.Longitude == nil || in.Latitude == nil && in.Longitude != nil {
		v.Add("latitude", "latitude and longitude must be provided together.",
			"يجب إدخال خط الطول وخط العرض معاً.")
	}
	if in.Latitude != nil && (*in.Latitude < -90 || *in.Latitude > 90) {
		v.Add("latitude", "latitude must be between -90 and 90.", "خط العرض يجب أن يكون بين -90 و 90.")
	}
	if in.Longitude != nil && (*in.Longitude < -180 || *in.Longitude > 180) {
		v.Add("longitude", "longitude must be between -180 and 180.", "خط الطول يجب أن يكون بين -180 و 180.")
	}
	if err := v.Err(); err != nil {
		return LocationView{}, err
	}

	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	loc := models.ProviderLocation{
		ProviderID:     userID,
		Name:           name,
		City:           city,
		District:       strPtr(validation.SanitizeText(in.District)),
		Street:         strPtr(validation.SanitizeText(in.Street)),
		AdditionalInfo: strPtr(validation.SanitizeText(in.AdditionalInfo)),
		Latitude:       in.Latitude,
		Longitude:      in.Longitude,
		IsActive:       active,
	}
	if err := p.svc.DB.WithContext(ctx).Create(&loc).Error; err != nil {
		return LocationView{}, apierr.Internal("Could not create the location.", "تعذر إنشاء الموقع.")
	}
	p.svc.audit(ctx, &userID, &userID, "provider.location.created", "location created", nil)
	return locationView(loc), nil
}

// ---------------------------------------------------------------------------
// Logo / commercial image (ERD Provider.logo_url)
// ---------------------------------------------------------------------------

// UploadLogo validates and stores the provider logo, then points
// provider.logo_url at the serving endpoint. Only JPG/PNG up to the global
// upload limit are accepted; the file lives in the private upload dir and is
// served through the authenticated-ish public logo endpoint (it is a
// non-sensitive commercial image — identity documents are NEVER public).
func (p *ProviderService) UploadLogo(ctx context.Context, userID uint, originalName, contentType string, content io.Reader) (string, error) {
	if _, err := p.getProvider(ctx, userID); err != nil {
		return "", err
	}
	if originalName == "" || len(originalName) > 255 {
		return "", apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "file", Message: "Invalid file name. / اسم الملف غير صالح.",
		}})
	}

	maxSize := p.svc.Cfg.Upload.MaxSizeBytes
	data, err := io.ReadAll(io.LimitReader(content, maxSize+1))
	if err != nil {
		return "", apierr.BadRequest("UPLOAD_READ_FAILED", "Could not read the uploaded file.", "تعذر قراءة الملف المرفوع.")
	}
	if int64(len(data)) > maxSize {
		return "", apierr.ErrValidation([]apierr.FieldViolation{{
			Field:   "file",
			Message: fmt.Sprintf("File exceeds the %d MB limit. / حجم الملف يتجاوز الحد المسموح.", maxSize/(1024*1024)),
		}})
	}
	sniffed := sniffContentType(data)
	if sniffed != "image/jpeg" && sniffed != "image/png" {
		return "", apierr.ErrValidation([]apierr.FieldViolation{{
			Field: "file", Message: "Only JPG or PNG logos are accepted. / يُقبل فقط شعار JPG أو PNG.",
		}})
	}
	ext := ".jpg"
	if sniffed == "image/png" {
		ext = ".png"
	}

	dir := filepath.Join(p.svc.Cfg.Upload.Dir, "logos")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", apierr.Internal("Could not store the logo.", "تعذر حفظ الشعار.")
	}
	fname := fmt.Sprintf("%d%s", userID, ext)
	if err := os.WriteFile(filepath.Join(dir, fname), data, 0o600); err != nil {
		return "", apierr.Internal("Could not store the logo.", "تعذر حفظ الشعار.")
	}

	// The serving route is registered under the configured API base path
	// (router.go: GET {base}/providers/{id}/logo); the stored URL must match
	// it or clients get the JSON 404 catch-all.
	logoURL := fmt.Sprintf("%s/providers/%d/logo", strings.TrimRight(p.svc.Cfg.HTTP.APIBasePath, "/"), userID)
	if err := p.svc.DB.WithContext(ctx).Model(&models.Provider{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{"logo_url": logoURL, "updated_at": time.Now().UTC()}).Error; err != nil {
		return "", apierr.Internal("Could not update the logo URL.", "تعذر تحديث رابط الشعار.")
	}
	p.svc.audit(ctx, &userID, &userID, "provider.logo.uploaded", "", nil)
	return logoURL, nil
}

// LogoFilePath returns the stored logo path + content type for serving.
// The stored logo_url carries no extension, so the actual file is probed
// (.png first, then .jpg) inside the private upload dir.
func (p *ProviderService) LogoFilePath(ctx context.Context, userID uint) (string, string, error) {
	if _, err := p.getProvider(ctx, userID); err != nil {
		return "", "", err
	}
	dir := filepath.Join(p.svc.Cfg.Upload.Dir, "logos")
	for _, probe := range []struct{ ext, ctype string }{
		{".png", "image/png"},
		{".jpg", "image/jpeg"},
	} {
		abs := filepath.Join(dir, fmt.Sprintf("%d%s", userID, probe.ext))
		if _, err := os.Stat(abs); err == nil {
			return abs, probe.ctype, nil
		}
	}
	return "", "", apierr.ErrNotFoundResource("Logo")
}

// ---------------------------------------------------------------------------
// Admin views
// ---------------------------------------------------------------------------

// AdminProviderRow is the admin listing of a provider account.
type AdminProviderRow struct {
	UserID              uint                      `json:"user_id"`
	FullName            string                    `json:"full_name"`
	PhoneNumber         string                    `json:"phone_number"`
	Status              models.AccountStatus      `json:"status"`
	VerificationStatus  models.VerificationStatus `json:"verification_status"`
	ProviderName        string                    `json:"provider_name"`
	ProviderDescription string                    `json:"provider_description,omitempty"`
	StartTime           string                    `json:"start_time,omitempty"`
	EndTime             string                    `json:"end_time,omitempty"`
	LogoURL             string                    `json:"logo_url,omitempty"`
	LocationCount       int                       `json:"location_count"`
	CreatedAt           time.Time                 `json:"created_at"`
}

// AdminListProviders returns providers filtered by account status ("" = all).
func (p *ProviderService) AdminListProviders(ctx context.Context, status string, limit, offset int) ([]AdminProviderRow, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	tx := p.svc.DB.WithContext(ctx).Model(&models.Provider{})
	if status != "" {
		if !models.AccountStatus(status).Valid() {
			return nil, 0, apierr.ErrValidation([]apierr.FieldViolation{{
				Field: "status", Message: "Unknown account status. / حالة حساب غير معروفة.",
			}})
		}
		tx = tx.Joins("JOIN users ON users.id = providers.user_id").
			Where("users.status = ?", status)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load providers.", "تعذر تحميل الموردين.")
	}

	var provs []models.Provider
	if err := tx.Preload("User").Preload("User.Role").
		Order("providers.created_at DESC").Limit(limit).Offset(offset).Find(&provs).Error; err != nil {
		return nil, 0, apierr.Internal("Could not load providers.", "تعذر تحميل الموردين.")
	}
	if len(provs) == 0 {
		return []AdminProviderRow{}, total, nil
	}
	ids := make([]uint, 0, len(provs))
	for _, pr := range provs {
		ids = append(ids, pr.UserID)
	}
	type lc struct {
		ProviderID uint
		N          int
	}
	var lcounts []lc
	_ = p.svc.DB.WithContext(ctx).Model(&models.ProviderLocation{}).
		Select("provider_id, COUNT(*) AS n").
		Where("provider_id IN ? AND deleted_at IS NULL", ids).
		Group("provider_id").Scan(&lcounts).Error
	locCount := map[uint]int{}
	for _, c := range lcounts {
		locCount[c.ProviderID] = c.N
	}

	rows := make([]AdminProviderRow, 0, len(provs))
	for _, pr := range provs {
		row := AdminProviderRow{
			UserID: pr.UserID, ProviderName: pr.ProviderName,
			LocationCount: locCount[pr.UserID], CreatedAt: pr.CreatedAt,
		}
		if pr.User != nil {
			row.FullName = pr.User.FullName
			row.PhoneNumber = pr.User.PhoneNumber
			row.Status = pr.User.Status
			row.VerificationStatus = pr.User.VerificationStatus
		}
		row.ProviderDescription = deref(pr.Description)
		row.StartTime = deref(pr.StartTime)
		row.EndTime = deref(pr.EndTime)
		row.LogoURL = deref(pr.LogoURL)
		rows = append(rows, row)
	}
	return rows, total, nil
}

// ProviderStats returns simple provider counters for the admin dashboard.
func (p *ProviderService) ProviderStats(ctx context.Context) (map[string]int64, error) {
	stats := map[string]int64{}
	var rows []struct {
		Status models.AccountStatus
		N      int64
	}
	err := p.svc.DB.WithContext(ctx).Raw(
		`SELECT users.status, COUNT(*) AS n
		   FROM providers JOIN users ON users.id = providers.user_id
		  GROUP BY users.status`).Scan(&rows).Error
	if err != nil {
		return nil, apierr.Internal("Could not load provider stats.", "تعذر تحميل إحصاءات الموردين.")
	}
	var total int64
	for _, r := range rows {
		stats[string(r.Status)] = r.N
		total += r.N
	}
	stats["total"] = total
	return stats, nil
}
