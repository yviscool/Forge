package domain

import (
	"errors"
	"strings"
)

// ValidateProblem 强制校验题面必要字段，与 SPEC 5.3 对齐。
func ValidateProblem(p Problem) error {
	if strings.TrimSpace(p.ContestID) == "" {
		return errors.New("contestId is required")
	}
	if strings.TrimSpace(p.Code) == "" {
		return errors.New("problem code/identifier is required")
	}
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(p.Statement) == "" {
		return errors.New("statement is required")
	}
	if strings.TrimSpace(p.Input) == "" {
		return errors.New("input format is required")
	}
	if strings.TrimSpace(p.Output) == "" {
		return errors.New("output format is required")
	}
	if strings.TrimSpace(p.Constraints) == "" {
		return errors.New("constraints specification is required")
	}
	return nil
}

// IsUserAllowed 参赛准入：无绑定名单时公开；否则个人名单或组交集命中其一即可。
func IsUserAllowed(c Contest, u User) bool {
	if len(c.GroupIDs) == 0 && len(c.ParticipantUserIDs) == 0 {
		return true
	}
	for _, id := range c.ParticipantUserIDs {
		if id == u.ID {
			return true
		}
	}
	inGroup := map[string]struct{}{}
	for _, g := range u.Groups {
		inGroup[g] = struct{}{}
	}
	for _, g := range c.GroupIDs {
		if _, ok := inGroup[g]; ok {
			return true
		}
	}
	return false
}

// NormalizeRole 非法角色回落为 student，保证入库值收敛。
func NormalizeRole(role string) string {
	switch role {
	case RoleAdmin, RoleTeacher, RoleStudent:
		return role
	default:
		return RoleStudent
	}
}
