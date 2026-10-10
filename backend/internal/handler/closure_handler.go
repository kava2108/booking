package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/service"
)

// ClosureHandler は休診・医師不在の設定（StaffOnly）。
// IMP 16-clinic-reservation-closures TASK-0001
type ClosureHandler struct{ svc service.ClosureService }

func NewClosureHandler(svc service.ClosureService) *ClosureHandler { return &ClosureHandler{svc: svc} }

type closureRequest struct {
	DoctorID *int64     `json:"doctor_id" validate:"omitempty,gt=0"`
	FromAt   *time.Time `json:"from_at" validate:"required"`
	ToAt     *time.Time `json:"to_at" validate:"required"`
	Reason   string     `json:"reason" validate:"max=200"`
}

type closureResultDTO struct {
	ClosedSlots int `json:"closed_slots"`
	Notified    int `json:"notified"`
}

// Create: POST /api/v1/staff/closures
func (h *ClosureHandler) Create(c echo.Context) error {
	actor, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	var req closureRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	res, err := h.svc.Create(c.Request().Context(), actor, service.ClosureInput{DoctorID: req.DoctorID, From: *req.FromAt, To: *req.ToAt, Reason: req.Reason})
	if err != nil {
		return serviceError(c, err)
	}
	return c.JSON(http.StatusCreated, closureResultDTO{ClosedSlots: res.ClosedSlots, Notified: res.Notified})
}

// Routes は /staff 配下に登録する。jwtAuth → staffOnly の順に適用する。
func (h *ClosureHandler) Routes(g *echo.Group, jwtAuth, staffOnly echo.MiddlewareFunc) {
	g.POST("/closures", h.Create, jwtAuth, staffOnly, noStore)
}
