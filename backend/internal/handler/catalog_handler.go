package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/service"
)

// CatalogHandler は認証不要の診療科・医師・空き枠 API。内部設定・定員などは返さない。
// IMP 11-clinic-reservation-slot-api-admin-settings TASK-0001
type CatalogHandler struct{ svc service.SlotService }

func NewCatalogHandler(svc service.SlotService) *CatalogHandler { return &CatalogHandler{svc: svc} }

type departmentDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type doctorDTO struct {
	ID           int64  `json:"id"`
	DepartmentID int64  `json:"department_id"`
	Name         string `json:"name"`
}

type slotDTO struct {
	ID                uuid.UUID `json:"id"`
	DoctorID          int64     `json:"doctor_id"`
	StartAt           time.Time `json:"start_at"`
	Available         bool      `json:"available"`
	UnavailableReason string    `json:"unavailable_reason,omitempty"`
}

func (h *CatalogHandler) Departments(c echo.Context) error {
	rows, err := h.svc.ListDepartments(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]departmentDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, departmentDTO{ID: r.ID, Name: r.Name})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *CatalogHandler) Doctors(c echo.Context) error {
	deptID, _, bad := queryInt64(c, "department_id", true)
	if bad != nil {
		return validationFailed(c, *bad)
	}
	rows, err := h.svc.ListDoctors(c.Request().Context(), deptID)
	if err != nil {
		return err
	}
	out := make([]doctorDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, doctorDTO{ID: r.ID, DepartmentID: r.DepartmentID, Name: r.Name})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *CatalogHandler) Slots(c echo.Context) error {
	var bad []apierror.Field
	deptID, _, f := queryInt64(c, "department_id", true)
	if f != nil {
		bad = append(bad, *f)
	}
	doctorID, hasDoctor, f := queryInt64(c, "doctor_id", false)
	if f != nil {
		bad = append(bad, *f)
	}
	var date time.Time
	if raw := c.QueryParam("date"); raw == "" {
		bad = append(bad, apierror.Field{Field: "date", Message: "is required"})
	} else if d, err := domain.ParseDate(raw); err != nil {
		bad = append(bad, apierror.Field{Field: "date", Message: "must be YYYY-MM-DD"})
	} else {
		date = d
	}
	if len(bad) > 0 {
		return validationFailed(c, bad...)
	}

	q := service.SlotQuery{DepartmentID: deptID, Date: date}
	if hasDoctor {
		q.DoctorID = &doctorID
	}
	views, err := h.svc.ListSlots(c.Request().Context(), q)
	if err != nil {
		return serviceError(c, err)
	}
	out := make([]slotDTO, 0, len(views))
	for _, v := range views {
		out = append(out, slotDTO{ID: v.ID, DoctorID: v.DoctorID, StartAt: v.StartAt, Available: v.Available, UnavailableReason: v.Reason})
	}
	return c.JSON(http.StatusOK, out)
}

// Routes は /api/v1 配下に公開ルートを登録する。
func (h *CatalogHandler) Routes(g *echo.Group, public echo.MiddlewareFunc) {
	g.GET("/departments", h.Departments, public)
	g.GET("/doctors", h.Doctors, public)
	g.GET("/slots", h.Slots, public)
}
