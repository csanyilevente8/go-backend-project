package httpapi

import (
	"strings"
	"testing"
)

func strptr(s string) *string { return &s }

func TestValidateTitleDescription(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description *string
		wantField   string // "" means valid
	}{
		{"valid", "Learn Go", strptr("something"), ""},
		{"valid nil description", "Learn Go", nil, ""},
		{"empty title", "", nil, "title"},
		{"whitespace-only title", "   ", nil, "title"},
		{"title at max 255", strings.Repeat("a", 255), nil, ""},
		{"title over 255", strings.Repeat("a", 256), nil, "title"},
		{"description at max 2000", "ok", strptr(strings.Repeat("d", 2000)), ""},
		{"description over 2000", "ok", strptr(strings.Repeat("d", 2001)), "description"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fe := validateTitleDescription(tc.title, tc.description)
			if tc.wantField == "" {
				if fe != nil {
					t.Fatalf("expected valid, got errors: %+v", fe)
				}
				return
			}
			if fe == nil {
				t.Fatalf("expected error on %q, got none", tc.wantField)
			}
			if _, ok := fe[tc.wantField]; !ok {
				t.Fatalf("expected error on field %q, got %+v", tc.wantField, fe)
			}
		})
	}
}

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"http://localhost:4200", "https://todo.leventeprojects.xyz"}
	cases := map[string]bool{
		"http://localhost:4200":            true,
		"https://todo.leventeprojects.xyz": true,
		"http://evil.example.com":          false,
		"":                                 false,
	}
	for origin, want := range cases {
		if got := originAllowed(origin, allowed); got != want {
			t.Errorf("originAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
}
