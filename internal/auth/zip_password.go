package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	zipPasswordSaltBytes = 16
	zipPasswordKeyBytes  = 32
	zipPasswordMemoryKiB = 32 * 1024
	zipPasswordTime      = 2
	zipPasswordThreads   = 2

	minZipPasswordRunes = 8
	maxZipPasswordRunes = 128
)

var (
	// ErrZipVerifyBusy prevents a burst of unlock attempts from multiplying
	// Argon2 memory use without bound. Callers should return a retryable error.
	ErrZipVerifyBusy = errors.New("exam password verifier is busy")

	// Capacity four bounds concurrent verification at roughly 128 MiB.
	zipVerifySlots = make(chan struct{}, 4)
)

// HashZipPassword creates a salted Argon2id verifier suitable for persistence.
// The returned PHC string contains no reversible copy of the password.
func HashZipPassword(password string) (string, error) {
	if n := len([]rune(password)); n < minZipPasswordRunes || n > maxZipPasswordRunes {
		return "", fmt.Errorf(
			"zip password must be between %d and %d characters",
			minZipPasswordRunes,
			maxZipPasswordRunes,
		)
	}
	salt := make([]byte, zipPasswordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate zip password salt: %w", err)
	}
	key := argon2.IDKey(
		[]byte(password),
		salt,
		zipPasswordTime,
		zipPasswordMemoryKiB,
		zipPasswordThreads,
		zipPasswordKeyBytes,
	)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		zipPasswordMemoryKiB,
		zipPasswordTime,
		zipPasswordThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyZipPassword checks a password against a stored PHC verifier. Malformed
// or dangerously expensive verifier parameters fail closed.
func VerifyZipPassword(verifier, password string) (bool, error) {
	params, err := parseZipPasswordVerifier(verifier)
	if err != nil {
		return false, err
	}
	select {
	case zipVerifySlots <- struct{}{}:
		defer func() { <-zipVerifySlots }()
	default:
		return false, ErrZipVerifyBusy
	}

	got := argon2.IDKey(
		[]byte(password),
		params.salt,
		params.time,
		params.memoryKiB,
		params.threads,
		uint32(len(params.key)),
	)
	return subtle.ConstantTimeCompare(got, params.key) == 1, nil
}

type zipPasswordVerifier struct {
	memoryKiB uint32
	time      uint32
	threads   uint8
	salt      []byte
	key       []byte
}

func parseZipPasswordVerifier(verifier string) (zipPasswordVerifier, error) {
	invalid := errors.New("invalid exam password verifier")
	parts := strings.Split(verifier, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return zipPasswordVerifier{}, invalid
	}

	paramParts := strings.Split(parts[3], ",")
	if len(paramParts) != 3 {
		return zipPasswordVerifier{}, invalid
	}
	values := make(map[string]uint64, 3)
	for _, part := range paramParts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return zipPasswordVerifier{}, invalid
		}
		if _, duplicate := values[kv[0]]; duplicate {
			return zipPasswordVerifier{}, invalid
		}
		n, err := strconv.ParseUint(kv[1], 10, 32)
		if err != nil {
			return zipPasswordVerifier{}, invalid
		}
		values[kv[0]] = n
	}
	memoryKiB, okM := values["m"]
	time, okT := values["t"]
	threads, okP := values["p"]
	if !okM || !okT || !okP ||
		memoryKiB < 8*1024 || memoryKiB > 64*1024 ||
		time < 1 || time > 4 ||
		threads < 1 || threads > 4 {
		return zipPasswordVerifier{}, invalid
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < zipPasswordSaltBytes || len(salt) > 64 {
		return zipPasswordVerifier{}, invalid
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != zipPasswordKeyBytes {
		return zipPasswordVerifier{}, invalid
	}
	return zipPasswordVerifier{
		memoryKiB: uint32(memoryKiB),
		time:      uint32(time),
		threads:   uint8(threads),
		salt:      salt,
		key:       key,
	}, nil
}
