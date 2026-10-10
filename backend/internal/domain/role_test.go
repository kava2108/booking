package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// IMP 6-clinic-reservation-rbac-audit-openapi TASK-0001: ロール値は users.role の CHECK 制約と一致する
func TestRole_Valid(t *testing.T) {
	for _, r := range []Role{RolePatient, RoleStaff, RoleClinicAdmin, RoleSystemAdmin} {
		assert.True(t, r.Valid(), r)
	}
	for _, r := range []Role{"", "user", "facility_admin", "admin", "PATIENT"} {
		assert.False(t, r.Valid(), "%q", r)
	}
}

func TestRole_Values(t *testing.T) {
	assert.Equal(t, "patient", string(RolePatient))
	assert.Equal(t, "staff", string(RoleStaff))
	assert.Equal(t, "clinic_admin", string(RoleClinicAdmin))
	assert.Equal(t, "system_admin", string(RoleSystemAdmin))
}
