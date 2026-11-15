package validation

import (
	"strings"
	"testing"
)

// --- phone normalization (Yemeni formats, SRS FR-CON-01) ---------------------

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"771234567":          "771234567",
		"0771234567":         "771234567",
		"+967771234567":      "771234567",
		"00967771234567":     "771234567",
		"771 234 567":        "771234567",
		"771-234-567":        "771234567",
		" +967 771 234 567 ": "771234567",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePhoneRejectsInvalid(t *testing.T) {
	invalid := []string{
		"", "   ", "12345", "881234567", // not starting with 7
		"77123456",     // too short
		"7712345678",   // too long
		"+12125551234", // non-Yemeni
		"96777123456",  // wrong length after stripping 967
		"abcdefghi",
	}
	for _, in := range invalid {
		if got := NormalizePhone(in); got != "" {
			t.Errorf("NormalizePhone(%q) = %q, want \"\"", in, got)
		}
	}
}

// --- sanitization (SRS NFR-SEC-05) -------------------------------------------

func TestSanitizeTextStripsInjectionPayloads(t *testing.T) {
	cases := map[string]string{
		"<script>alert(1)</script>":    "scriptalert(1)/script",
		"  Ahmed  ":                    "Ahmed",
		"a\x00b":                       "ab",
		"<img src=x onerror=alert(1)>": "img src=x onerror=alert(1)",
		"normal text":                  "normal text",
	}
	for in, want := range cases {
		if got := SanitizeText(in); got != want {
			t.Errorf("SanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeTextKeepsArabic(t *testing.T) {
	in := "  موارد للتوصيل  "
	if got := SanitizeText(in); got != "موارد للتوصيل" {
		t.Errorf("SanitizeText dropped Arabic content: %q", got)
	}
}

// --- password policy ---------------------------------------------------------

func TestPasswordPolicy(t *testing.T) {
	var v Violations
	if !v.Password("password", "abcdef12") {
		t.Errorf("valid password rejected: %v", v)
	}
	if len(v) != 0 {
		t.Fatalf("unexpected violations: %v", v)
	}

	weak := []string{"short1", "12345678", "abcdefgh", "abc12"}
	for _, pw := range weak {
		var v2 Violations
		if v2.Password("password", pw) {
			t.Errorf("weak password %q accepted", pw)
		}
		if err := v2.Err(); err == nil {
			t.Errorf("no error for weak password %q", pw)
		} else if err.Code != "VALIDATION_ERROR" {
			t.Errorf("code = %q, want VALIDATION_ERROR", err.Code)
		}
	}
}

// --- time format (SRS NFR-DAT-01) --------------------------------------------

func TestTimeHHMM(t *testing.T) {
	valid := []string{"00:00", "08:30", "23:59", "17:05"}
	for _, tm := range valid {
		var v Violations
		if !v.TimeHHMM("start_time", tm) {
			t.Errorf("%q rejected as invalid time", tm)
		}
	}
	invalid := []string{"24:00", "8:30", "08:60", "0830", "abc", ""}
	for _, tm := range invalid {
		var v Violations
		if v.TimeHHMM("start_time", tm) {
			t.Errorf("%q accepted as valid time", tm)
		}
	}
}

// --- misc validators ---------------------------------------------------------

func TestRequiredAndOneOfAndDayOfWeek(t *testing.T) {
	var v Violations
	v.Required("name", "   ")
	v.OneOf("type", "gas", []string{"home", "business"})
	v.DayOfWeek("day", 9)
	violations := v.Err()
	if violations == nil {
		t.Fatal("expected violations")
	}
	if violations.Status != 400 || violations.Code != "VALIDATION_ERROR" {
		t.Errorf("unexpected error: %d %s", violations.Status, violations.Code)
	}
	// Arabic text must be present in messages (NFR-USA-02).
	if !strings.Contains(violations.MessageAr, "الحقل") && !strings.Contains(violations.MessageAr, "غير") {
		t.Errorf("expected Arabic message, got %q", violations.MessageAr)
	}
}

func TestTextLengthCountsRunes(t *testing.T) {
	var v Violations
	v.TextLength("desc", "موارد", 2, 500) // 5 runes — OK
	if err := v.Err(); err != nil {
		t.Errorf("Arabic text wrongly rejected: %v", err)
	}
	var v2 Violations
	v2.TextLength("desc", strings.Repeat("a", 501), 2, 500)
	if err := v2.Err(); err == nil {
		t.Error("501 chars accepted for max 500")
	}
}
