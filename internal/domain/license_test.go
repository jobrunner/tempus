package domain_test

import (
	"testing"

	"github.com/jobrunner/tempus/internal/domain"
)

func TestLicenseValidate(t *testing.T) {
	full := domain.License{Name: "CC BY 4.0", URL: "https://example.org/l", Attribution: "Example"}

	tests := []struct {
		name    string
		lic     domain.License
		wantErr bool
	}{
		{"complete", full, false},
		{"missing name", domain.License{URL: full.URL, Attribution: full.Attribution}, true},
		{"missing url", domain.License{Name: full.Name, Attribution: full.Attribution}, true},
		{"missing attribution", domain.License{Name: full.Name, URL: full.URL}, true},
		{"all empty", domain.License{}, true},
		{"whitespace only", domain.License{Name: " ", URL: "\t", Attribution: "\n"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.lic.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error for %+v", tc.lic)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

// The error must name the missing fields — a bare "invalid license" sends the
// operator hunting through every provider.
func TestLicenseValidateNamesMissingFields(t *testing.T) {
	err := domain.License{Name: "x"}.Validate()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"url", "attribution"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q does not name the missing field %q", err, want)
		}
	}
	if contains(err.Error(), "name") {
		t.Errorf("error %q names %q, which was present", err, "name")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
