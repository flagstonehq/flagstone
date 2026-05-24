package auth

import "testing"

func TestRoleLevel(t *testing.T) {
	tests := []struct {
		role  Role
		level int
	}{
		{RoleOwner, 4},
		{RoleAdmin, 3},
		{RoleMember, 2},
		{RoleViewer, 1},
		{Role("weird"), 0},
		{Role(""), 0},
	}

	for _, tt := range tests {
		if got := tt.role.Level(); got != tt.level {
			t.Errorf("Role(%q).Level() = %d, want %d", tt.role, got, tt.level)
		}
	}
}

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		name    string
		role    Role
		minimum Role
		want    bool
	}{
		{"owner >= admin", RoleOwner, RoleAdmin, true},
		{"owner >= viewer", RoleOwner, RoleViewer, true},
		{"admin >= admin", RoleAdmin, RoleAdmin, true},
		{"member >= viewer", RoleMember, RoleViewer, true},
		{"viewer !>= member", RoleViewer, RoleMember, false},
		{"viewer !>= admin", RoleViewer, RoleAdmin, false},
		{"unknown !>= viewer", Role("unknown"), RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.AtLeast(tt.minimum); got != tt.want {
				t.Errorf("%s.AtLeast(%s) = %t, want %t", tt.role, tt.minimum, got, tt.want)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input string
		want  Role
	}{
		{"owner", RoleOwner},
		{"OWNER", RoleOwner},
		{"Owner", RoleOwner},
		{"  admin  ", RoleAdmin},
		{"member", RoleMember},
		{"viewer", RoleViewer},
		{"", Role("")},
	}

	for _, tt := range tests {
		if got := ParseRole(tt.input); got != tt.want {
			t.Errorf("ParseRole(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRoleValid(t *testing.T) {
	if !RoleOwner.Valid() {
		t.Error("expected owner to be valid")
	}
	if !RoleAdmin.Valid() {
		t.Error("expected admin to be valid")
	}
	if !RoleMember.Valid() {
		t.Error("expected member to be valid")
	}
	if !RoleViewer.Valid() {
		t.Error("expected viewer to be valid")
	}
	if Role("").Valid() {
		t.Error("expected empty role to be invalid")
	}
	if Role("hacker").Valid() {
		t.Error("expected unknown role to be invalid")
	}
}
