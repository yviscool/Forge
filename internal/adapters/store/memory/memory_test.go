package memory

import (
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/storetest"
)

func TestStoreConformance(t *testing.T) {
	storetest.Exercise(t, New())
	storetest.PasswordsAndSessions(t, New())
	storetest.TransactionSemantics(t, New())
	storetest.ProvinceStates(t, New())
	storetest.CodeVersions(t, New())
	storetest.Tickets(t, New())
	storetest.DeleteUserCascade(t, New())
}
