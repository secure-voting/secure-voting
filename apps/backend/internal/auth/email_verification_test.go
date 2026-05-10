package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestDevEmailVerificationSender_ReturnsDevDelivery(t *testing.T) {
	sender := NewDevEmailVerificationSender()

	delivery, err := sender.SendEmailVerificationCode(
		"voter@example.com",
		"ABCD-EFGH-JKLM-NPQR",
		"2026-04-29T10:00:00Z",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delivery != "dev" {
		t.Fatalf("expected dev delivery, got %q", delivery)
	}
}

func TestDisabledEmailVerificationSender_ReturnsConfigError(t *testing.T) {
	sender := NewDisabledEmailVerificationSender()

	_, err := sender.SendEmailVerificationCode(
		"voter@example.com",
		"ABCD-EFGH-JKLM-NPQR",
		"2026-04-29T10:00:00Z",
	)
	if !errors.Is(err, errEmailDeliveryNotConfigured) {
		t.Fatalf("expected errEmailDeliveryNotConfigured, got %v", err)
	}
}

func TestBuildEmailVerificationMessage_ContainsCodeAndExpiration(t *testing.T) {
	msg := buildEmailVerificationMessage("ABCD-EFGH-JKLM-NPQR", "2026-04-29T10:00:00Z")

	if !strings.Contains(msg, "ABCD-EFGH-JKLM-NPQR") {
		t.Fatalf("expected message to contain verification code, got %q", msg)
	}
	if !strings.Contains(msg, "2026-04-29T10:00:00Z") {
		t.Fatalf("expected message to contain expiration, got %q", msg)
	}
}
