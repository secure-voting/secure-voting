package adminsettings

import (
	"context"
	"testing"
)

func TestValidTLSMode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"disabled", true},
		{"lets_encrypt", true},
		{"custom", true},
		{"unknown", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := validTLSMode(tc.in); got != tc.want {
			t.Fatalf("validTLSMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestValidEmailOrEmpty(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"admin@example.com", true},
		{"bad", false},
	}

	for _, tc := range cases {
		if got := validEmailOrEmpty(tc.in); got != tc.want {
			t.Fatalf("validEmailOrEmpty(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestUpdate_ValidationOnly(t *testing.T) {
	svc := NewService(nil)

	if _, code, err := svc.Update(context.Background(), UpdateInput{}); err != nil || code != "unauthorized" {
		t.Fatalf("expected unauthorized, got code=%q err=%v", code, err)
	}

	if _, code, err := svc.Update(context.Background(), UpdateInput{
		ActorUserID: "u1",
		TLSMode:     "wrong",
	}); err != nil || code != "invalid_tls_mode" {
		t.Fatalf("expected invalid_tls_mode, got code=%q err=%v", code, err)
	}

	if _, code, err := svc.Update(context.Background(), UpdateInput{
		ActorUserID:     "u1",
		TLSMode:         "custom",
		TLSContactEmail: "bad-email",
	}); err != nil || code != "invalid_tls_contact_email" {
		t.Fatalf("expected invalid_tls_contact_email, got code=%q err=%v", code, err)
	}

	zero := 0
	if _, code, err := svc.Update(context.Background(), UpdateInput{
		ActorUserID:         "u1",
		TLSMode:             "custom",
		TLSContactEmail:     "admin@example.com",
		BackupRetentionDays: &zero,
	}); err != nil || code != "invalid_backup_retention_days" {
		t.Fatalf("expected invalid_backup_retention_days, got code=%q err=%v", code, err)
	}
}