package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContestJSONDerivesProtectionAndNeverExposesVerifier(t *testing.T) {
	raw, err := json.Marshal(Contest{
		ID:              "cnt-1",
		Name:            "Final",
		ZipPasswordHash: "$argon2id$secret-verifier",
		ZipProtected:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "secret-verifier") ||
		strings.Contains(text, "zipPasswordHash") ||
		strings.Contains(text, "zipPassword") {
		t.Fatalf("contest wire format leaked the verifier: %s", text)
	}
	if !strings.Contains(text, `"zipProtected":true`) {
		t.Fatalf("contest wire format did not derive zipProtected: %s", text)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["zipProtected"] != true {
		t.Fatalf("zipProtected missing after decode: %#v", decoded)
	}
}
