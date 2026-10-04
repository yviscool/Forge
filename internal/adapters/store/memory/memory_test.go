package memory

import (
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/storetest"
)

func TestStoreConformance(t *testing.T) {
	storetest.Exercise(t, New())
	storetest.PasswordsAndSessions(t, New())
}
