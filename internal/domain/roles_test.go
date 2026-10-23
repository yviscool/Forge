package domain

import "testing"

func TestRoleManagementMatrix(t *testing.T) {
	tests := []struct {
		name      string
		actor     string
		requested string
		assign    bool
	}{
		{name: "teacher creates student", actor: RoleTeacher, requested: RoleStudent, assign: true},
		{name: "teacher cannot create teacher", actor: RoleTeacher, requested: RoleTeacher},
		{name: "teacher cannot create admin", actor: RoleTeacher, requested: RoleAdmin},
		{name: "admin creates student", actor: RoleAdmin, requested: RoleStudent, assign: true},
		{name: "admin creates teacher", actor: RoleAdmin, requested: RoleTeacher, assign: true},
		{name: "admin creates admin", actor: RoleAdmin, requested: RoleAdmin, assign: true},
		{name: "invalid role rejected", actor: RoleAdmin, requested: "owner"},
		{name: "student cannot assign", actor: RoleStudent, requested: RoleStudent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAssignRole(tt.actor, tt.requested); got != tt.assign {
				t.Fatalf("CanAssignRole(%q, %q) = %v, want %v",
					tt.actor, tt.requested, got, tt.assign)
			}
		})
	}
}

func TestCanManageUser(t *testing.T) {
	tests := []struct {
		name   string
		actor  string
		target string
		want   bool
	}{
		{name: "teacher manages student", actor: RoleTeacher, target: RoleStudent, want: true},
		{name: "teacher cannot manage teacher", actor: RoleTeacher, target: RoleTeacher},
		{name: "teacher cannot manage admin", actor: RoleTeacher, target: RoleAdmin},
		{name: "admin manages student", actor: RoleAdmin, target: RoleStudent, want: true},
		{name: "admin manages teacher", actor: RoleAdmin, target: RoleTeacher, want: true},
		{name: "admin manages admin", actor: RoleAdmin, target: RoleAdmin, want: true},
		{name: "student cannot manage", actor: RoleStudent, target: RoleStudent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanManageUser(tt.actor, tt.target); got != tt.want {
				t.Fatalf("CanManageUser(%q, %q) = %v, want %v",
					tt.actor, tt.target, got, tt.want)
			}
		})
	}
}
