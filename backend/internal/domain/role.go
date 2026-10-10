// Package domain はアプリ全体で共有するドメイン型を定義する。
package domain

// Role は users.role の値。DB の CHECK 制約（migration 000001）と一致させること。
type Role string

const (
	RolePatient     Role = "patient"
	RoleStaff       Role = "staff"
	RoleClinicAdmin Role = "clinic_admin"
	RoleSystemAdmin Role = "system_admin"
)

// Valid は定義済みのロール値かを返す。
func (r Role) Valid() bool {
	switch r {
	case RolePatient, RoleStaff, RoleClinicAdmin, RoleSystemAdmin:
		return true
	}
	return false
}
