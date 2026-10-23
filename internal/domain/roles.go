package domain

import "strings"

// ValidRole reports whether a value is one of the three canonical roles. Write
// paths use this instead of NormalizeRole so a typo cannot silently create a
// student with elevated expectations.
func ValidRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin, RoleTeacher, RoleStudent:
		return true
	default:
		return false
	}
}

// CanAssignRole is the account-management privilege matrix. Teachers may
// manage candidates; only administrators may create or promote staff.
func CanAssignRole(actorRole, requestedRole string) bool {
	actor := NormalizeRole(actorRole)
	requested := NormalizeRole(requestedRole)
	if !ValidRole(requestedRole) {
		return false
	}
	switch actor {
	case RoleAdmin:
		return true
	case RoleTeacher:
		return requested == RoleStudent
	default:
		return false
	}
}

// CanManageUser controls password resets and other destructive account
// operations against an existing account.
func CanManageUser(actorRole, targetRole string) bool {
	actor := NormalizeRole(actorRole)
	target := NormalizeRole(targetRole)
	switch actor {
	case RoleAdmin:
		return true
	case RoleTeacher:
		return target == RoleStudent
	default:
		return false
	}
}
