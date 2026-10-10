package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/service"
)

// WaitTimeNote は待ち時間表示に必ず併記する注記（REQ-004-AC-6）。
const WaitTimeNote = "待ち時間は目安です。混雑状況により変動します"

// WaitTimeHandler は待ち時間の公開 API と、予約患者本人の順番 API。
// 公開応答は件数と分のみで、個人を特定できる情報（氏名・受付番号など）を含めない（REQ-007-AC-1）。
// IMP 15-clinic-reservation-wait-time-api TASK-0001
type WaitTimeHandler struct{ svc service.WaitTimeService }

func NewWaitTimeHandler(svc service.WaitTimeService) *WaitTimeHandler {
	return &WaitTimeHandler{svc: svc}
}

type departmentWaitDTO struct {
	DepartmentID int64  `json:"department_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	// 算出不可のときは数値を一切返さない（推測しない）。ポインタ + omitempty で 0 分と区別する
	WaitingCount *int   `json:"waiting_count,omitempty"`
	WaitMinutes  *int   `json:"wait_minutes,omitempty"`
	Basis        string `json:"basis,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type waitTimesDTO struct {
	UpdatedAt   time.Time           `json:"updated_at"`
	Note        string              `json:"note"`
	Departments []departmentWaitDTO `json:"departments"`
}

type positionDTO struct {
	Status          string     `json:"status"`
	AheadCount      int        `json:"ahead_count"`
	EstimatedCallAt *time.Time `json:"estimated_call_at"` // 算出できないときは null
	Basis           string     `json:"basis,omitempty"`
}

// WaitTimes: GET /api/v1/wait-times（認証不要）
func (h *WaitTimeHandler) WaitTimes(c echo.Context) error {
	res, err := h.svc.Get(c.Request().Context())
	if err != nil {
		return err
	}
	out := waitTimesDTO{UpdatedAt: res.UpdatedAt, Note: WaitTimeNote, Departments: make([]departmentWaitDTO, 0, len(res.Departments))}
	for _, d := range res.Departments {
		dto := departmentWaitDTO{DepartmentID: d.DepartmentID, Name: d.Name, Status: d.Status, Reason: d.Reason, Basis: d.Basis}
		if d.Status == service.WaitStatusAvail {
			count, minutes := d.WaitingCount, d.WaitMinutes
			dto.WaitingCount, dto.WaitMinutes = &count, &minutes
		}
		out.Departments = append(out.Departments, dto)
	}
	// 共有キャッシュ可（サーバー側 10 秒キャッシュと同じ期間）
	c.Response().Header().Set("Cache-Control", "public, max-age=10")
	return c.JSON(http.StatusOK, out)
}

// Position: GET /api/v1/appointments/{id}/position（本人のみ）
func (h *WaitTimeHandler) Position(c echo.Context) error {
	pid, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	id, ok := parseID(c)
	if !ok {
		return apierror.Write(c, http.StatusNotFound, "not_found", "appointment not found")
	}
	p, err := h.svc.Position(c.Request().Context(), pid, id)
	if err != nil {
		return appointmentError(c, err)
	}
	return c.JSON(http.StatusOK, positionDTO{Status: p.Status, AheadCount: p.AheadCount, EstimatedCallAt: p.EstimatedCallAt, Basis: p.Basis})
}

// Routes: 待ち時間は公開（public はレートリミット）、順番は認証必須。
func (h *WaitTimeHandler) Routes(g *echo.Group, public, jwtAuth echo.MiddlewareFunc) {
	g.GET("/wait-times", h.WaitTimes, public)
	if jwtAuth != nil {
		g.GET("/appointments/:id/position", h.Position, jwtAuth, noStore)
	}
}
