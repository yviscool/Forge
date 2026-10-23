package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestZipPasswordVerifierRoundTripAndSalting(t *testing.T) {
	const password = "NOIP-2026-PASSWORD"

	first, err := HashZipPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashZipPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("equal passwords must produce independently salted verifiers")
	}
	if strings.Contains(first, password) {
		t.Fatal("verifier contains the plaintext password")
	}
	if !strings.HasPrefix(first, "$argon2id$") {
		t.Fatalf("unexpected verifier format: %q", first)
	}

	ok, err := VerifyZipPassword(first, password)
	if err != nil || !ok {
		t.Fatalf("correct password rejected: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyZipPassword(first, "wrong-password")
	if err != nil || ok {
		t.Fatalf("wrong password accepted: ok=%v err=%v", ok, err)
	}
}

func TestZipPasswordVerifierRejectsMalformedOrUnsafeParameters(t *testing.T) {
	for _, verifier := range []string{
		"",
		"plaintext",
		"$argon2id$v=19$m=32768,t=2,p=2$bad$bad",
		"$argon2id$v=19$m=1073741824,t=2,p=2$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		"$argon2id$v=19$m=32768,t=0,p=2$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
	} {
		if _, err := VerifyZipPassword(verifier, "password"); err == nil {
			t.Fatalf("malformed verifier was accepted: %q", verifier)
		}
	}
}

func TestZipPasswordVerifierBusyFailsClosed(t *testing.T) {
	verifier, err := HashZipPassword("NOIP-2026-PASSWORD")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < cap(zipVerifySlots); i++ {
		zipVerifySlots <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < cap(zipVerifySlots); i++ {
			<-zipVerifySlots
		}
	})

	if _, err := VerifyZipPassword(verifier, "NOIP-2026-PASSWORD"); !errors.Is(err, ErrZipVerifyBusy) {
		t.Fatalf("saturated verifier pool must fail closed, got %v", err)
	}
}
