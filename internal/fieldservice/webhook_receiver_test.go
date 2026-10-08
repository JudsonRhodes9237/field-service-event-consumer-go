package fieldservice

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"event_id":"evt-1001","event_type":"photo_uploaded"}`)
	secret := "local-demo-secret"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if err := verifySignature(body, sig, secret); err != nil {
		t.Fatalf("verifySignature() error = %v, want nil", err)
	}

	if err := verifySignature(body, "sha256=deadbeef", secret); err == nil {
		t.Fatal("verifySignature() with bad signature = nil, want error")
	}
}
