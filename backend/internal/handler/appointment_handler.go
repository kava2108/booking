package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/middleware"
	"github.com/kava2108/booking/backend/internal/service"
)

// AppointmentHandler は患者の予約 API。すべて認証必須で、操作対象は常に本人の予約（他人の予約は 404）。
// IMP 12-clinic-reservation-appointment-api TASK-0001
type AppointmentHandler struct{ svc service.AppointmentService }

func NewAppointmentHandler(svc service.AppointmentService) *AppointmentHandler {
	return &AppointmentHandler{svc: svc}
}

type createAppointmentRequest struct {
	SlotID      string `json:"slot_id" validate:"required,uuid"`
	VisitType   string `json:"visit_type" validate:"required,oneof=first revisit"`
	SymptomNote string `json:"symptom_note" validate:"max=1000"`
}

type rescheduleRequest struct {
	SlotID string `json:"slot_id" validate:"required,uuid"`
}

type appointmentDTO struct {
	ID            uuid.UUID `json:"id"`
	AppointmentNo string    `json:"appointment_no"`
	SlotStart     time.Time `json:"slot_start"`
	Department    string    `json:"department"`
	Doctor        string    `json:"doctor"`
}

// appointmentSummaryDTO は本人向け。symptom_note を含むのでスタッフ・公開向けの DTO としては使わない。
type appointmentSummaryDTO struct {
	ID            uuid.UUID `json:"id"`
	AppointmentNo string    `json:"appointment_no"`
	SlotStart     time.Time `json:"slot_start"`
	Department    string    `json:"department"`
	Doctor        string    `json:"doctor"`
	VisitType     string    `json:"visit_type"`
	Status        string    `json:"status"`
	SymptomNote   string    `json:"symptom_note,omitempty"`
	Cancelable    bool      `json:"cancelable"`
}

func toAppointmentDTO(r *service.AppointmentResult) appointmentDTO {
	return appointmentDTO{ID: r.ID, AppointmentNo: r.AppointmentNo, SlotStart: r.SlotStart, Department: r.Department, Doctor: r.Doctor}
}

// patientID は認証済みユーザーの ID。JWT ミドルウェアが前段にあるため通常は常に存在する。
func patientID(c echo.Context) (uuid.UUID, bool) {
	id, ok := middleware.IdentityFrom(c)
	return id.UserID, ok
}

// appointmentError は予約 service のエラーを HTTP 応答に変換する。
func appointmentError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrSlotFull):
		return apierror.Write(c, http.StatusConflict, "slot_full", "この枠は満席になりました。別の枠をお選びください")
	case errors.Is(err, service.ErrDuplicateAppointment):
		return apierror.Write(c, http.StatusConflict, "duplicate_appointment", "同じ時間帯に既に予約があります")
	case errors.Is(err, service.ErrCancelDeadlinePassed):
		return apierror.Write(c, http.StatusUnprocessableEntity, "cancel_deadline_passed", "キャンセル・変更の期限を過ぎています。医療機関へお電話ください")
	case errors.Is(err, service.ErrAppointmentNotFound):
		return apierror.Write(c, http.StatusNotFound, "not_found", "appointment not found")
	}
	return serviceError(c, err)
}

// parseID はパスの UUID を読む。不正な値は存在しない ID と同じ 404 にする。
func parseID(c echo.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	return id, err == nil
}

// Create: POST /api/v1/appointments
func (h *AppointmentHandler) Create(c echo.Context) error {
	pid, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	var req createAppointmentRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	slot, _ := uuid.Parse(req.SlotID) // validate:"uuid" で検証済み
	res, err := h.svc.Create(c.Request().Context(), pid, service.CreateAppointmentInput{SlotID: slot, VisitType: req.VisitType, SymptomNote: req.SymptomNote})
	if err != nil {
		return appointmentError(c, err)
	}
	return c.JSON(http.StatusCreated, toAppointmentDTO(res))
}

// ListMine: GET /api/v1/appointments/me
func (h *AppointmentHandler) ListMine(c echo.Context) error {
	pid, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	rows, err := h.svc.ListMine(c.Request().Context(), pid)
	if err != nil {
		return appointmentError(c, err)
	}
	out := make([]appointmentSummaryDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, appointmentSummaryDTO{
			ID: r.ID, AppointmentNo: r.AppointmentNo, SlotStart: r.SlotStart, Department: r.Department, Doctor: r.Doctor,
			VisitType: r.VisitType, Status: r.Status, SymptomNote: r.SymptomNote, Cancelable: r.Cancelable,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// Reschedule: PATCH /api/v1/appointments/{id}
func (h *AppointmentHandler) Reschedule(c echo.Context) error {
	pid, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	id, ok := parseID(c)
	if !ok {
		return apierror.Write(c, http.StatusNotFound, "not_found", "appointment not found")
	}
	var req rescheduleRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	slot, _ := uuid.Parse(req.SlotID)
	res, err := h.svc.Reschedule(c.Request().Context(), pid, id, slot)
	if err != nil {
		return appointmentError(c, err)
	}
	return c.JSON(http.StatusOK, toAppointmentDTO(res))
}

// Cancel: DELETE /api/v1/appointments/{id}
func (h *AppointmentHandler) Cancel(c echo.Context) error {
	pid, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	id, ok := parseID(c)
	if !ok {
		return apierror.Write(c, http.StatusNotFound, "not_found", "appointment not found")
	}
	if err := h.svc.Cancel(c.Request().Context(), pid, id); err != nil {
		return appointmentError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// Routes は /api/v1 配下に登録する。個人情報（症状メモ）を含む応答はキャッシュさせない。
func (h *AppointmentHandler) Routes(g *echo.Group, jwtAuth echo.MiddlewareFunc) {
	g.POST("/appointments", h.Create, jwtAuth, noStore)
	g.GET("/appointments/me", h.ListMine, jwtAuth, noStore)
	g.PATCH("/appointments/:id", h.Reschedule, jwtAuth, noStore)
	g.DELETE("/appointments/:id", h.Cancel, jwtAuth, noStore)
}
