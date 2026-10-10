package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/service"
)

// AdminHandler はクリニック管理者向けの設定 API（AdminOnly で保護する）。
// IMP 11-clinic-reservation-slot-api-admin-settings TASK-0001
type AdminHandler struct{ svc service.AdminService }

func NewAdminHandler(svc service.AdminService) *AdminHandler { return &AdminHandler{svc: svc} }

type scheduleRequest struct {
	Weekday     int    `json:"weekday" validate:"gte=0,lte=6"`
	StartTime   string `json:"start_time" validate:"required"`
	EndTime     string `json:"end_time" validate:"required"`
	SlotMinutes int    `json:"slot_minutes" validate:"required,gte=1"`
	Capacity    int    `json:"capacity" validate:"required,gte=1"`
}

type putSchedulesRequest struct {
	DoctorID  int64             `json:"doctor_id" validate:"required,gt=0"`
	Schedules []scheduleRequest `json:"schedules" validate:"required,dive"`
}

type deptSettingsRequest struct {
	ID                     int64   `json:"id" validate:"required,gt=0"`
	StandardConsultMinutes *int    `json:"standard_consult_minutes"`
	WalkinCutoff           *string `json:"walkin_cutoff"`
	WalkinCapacity         *int    `json:"walkin_capacity"`
}

type putSettingsRequest struct {
	CancelDeadlineMinutes *int                  `json:"cancel_deadline_minutes"`
	BookingWindowDays     *int                  `json:"booking_window_days"`
	Departments           []deptSettingsRequest `json:"departments" validate:"omitempty,dive"`
}

type settingsResponse struct {
	CancelDeadlineMinutes int `json:"cancel_deadline_minutes"`
	BookingWindowDays     int `json:"booking_window_days"`
}

// PutSchedules: PUT /api/v1/admin/schedules
func (h *AdminHandler) PutSchedules(c echo.Context) error {
	var req putSchedulesRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	in := service.PutSchedulesInput{DoctorID: req.DoctorID}
	for _, s := range req.Schedules {
		in.Schedules = append(in.Schedules, service.ScheduleInput{
			Weekday: s.Weekday, StartTime: s.StartTime, EndTime: s.EndTime, SlotMinutes: s.SlotMinutes, Capacity: s.Capacity,
		})
	}
	if err := h.svc.PutSchedules(c.Request().Context(), in); err != nil {
		return serviceError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

// PutSettings: PUT /api/v1/admin/settings
func (h *AdminHandler) PutSettings(c echo.Context) error {
	var req putSettingsRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	in := service.SettingsInput{CancelDeadlineMinutes: req.CancelDeadlineMinutes, BookingWindowDays: req.BookingWindowDays}
	for _, d := range req.Departments {
		in.Departments = append(in.Departments, service.DepartmentSettingsInput{
			ID: d.ID, StandardConsultMinutes: d.StandardConsultMinutes, WalkinCutoff: d.WalkinCutoff, WalkinCapacity: d.WalkinCapacity,
		})
	}
	out, err := h.svc.PutSettings(c.Request().Context(), in)
	if err != nil {
		return serviceError(c, err)
	}
	return c.JSON(http.StatusOK, settingsResponse{CancelDeadlineMinutes: out.CancelDeadlineMinutes, BookingWindowDays: out.BookingWindowDays})
}

type deptSettingsDTO struct {
	ID                     int64   `json:"id"`
	Name                   string  `json:"name"`
	StandardConsultMinutes *int    `json:"standard_consult_minutes"`
	WalkinCutoff           *string `json:"walkin_cutoff"`
	WalkinCapacity         *int    `json:"walkin_capacity"`
}

type settingsDetailResponse struct {
	CancelDeadlineMinutes int               `json:"cancel_deadline_minutes"`
	BookingWindowDays     int               `json:"booking_window_days"`
	Departments           []deptSettingsDTO `json:"departments"`
}

type schedulesResponse struct {
	DoctorID  int64             `json:"doctor_id"`
	Schedules []scheduleRequest `json:"schedules"`
}

// GetSettings: GET /api/v1/admin/settings（管理者画面の現在値）
func (h *AdminHandler) GetSettings(c echo.Context) error {
	v, err := h.svc.GetSettings(c.Request().Context())
	if err != nil {
		return serviceError(c, err)
	}
	out := settingsDetailResponse{CancelDeadlineMinutes: v.CancelDeadlineMinutes, BookingWindowDays: v.BookingWindowDays, Departments: make([]deptSettingsDTO, 0, len(v.Departments))}
	for _, d := range v.Departments {
		out.Departments = append(out.Departments, deptSettingsDTO{ID: d.ID, Name: d.Name, StandardConsultMinutes: d.StandardConsultMinutes, WalkinCutoff: d.WalkinCutoff, WalkinCapacity: d.WalkinCapacity})
	}
	return c.JSON(http.StatusOK, out)
}

// GetSchedules: GET /api/v1/admin/schedules?doctor_id=
func (h *AdminHandler) GetSchedules(c echo.Context) error {
	doctorID, _, bad := queryInt64(c, "doctor_id", true)
	if bad != nil {
		return validationFailed(c, *bad)
	}
	rows, err := h.svc.GetSchedules(c.Request().Context(), doctorID)
	if err != nil {
		return serviceError(c, err)
	}
	out := schedulesResponse{DoctorID: doctorID, Schedules: make([]scheduleRequest, 0, len(rows))}
	for _, r := range rows {
		out.Schedules = append(out.Schedules, scheduleRequest{Weekday: r.Weekday, StartTime: r.StartTime, EndTime: r.EndTime, SlotMinutes: r.SlotMinutes, Capacity: r.Capacity})
	}
	return c.JSON(http.StatusOK, out)
}

// Routes は /api/v1/admin 配下を登録する。jwtAuth → adminOnly の順で適用する。
func (h *AdminHandler) Routes(g *echo.Group, jwtAuth, adminOnly echo.MiddlewareFunc) {
	g.GET("/schedules", h.GetSchedules, jwtAuth, adminOnly, noStore)
	g.PUT("/schedules", h.PutSchedules, jwtAuth, adminOnly)
	g.GET("/settings", h.GetSettings, jwtAuth, adminOnly, noStore)
	g.PUT("/settings", h.PutSettings, jwtAuth, adminOnly)
}
