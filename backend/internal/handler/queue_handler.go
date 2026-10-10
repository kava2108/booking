package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/service"
)

// QueueHandler は来院受付・呼出・診察・当日受付と、スタッフ向け待機列。
// /staff/** は StaffOnly、待機列の参照（氏名・症状メモ）は監査ログに記録する。
// IMP 13-clinic-reservation-queue-staff-walkin TASK-0001
type QueueHandler struct{ svc service.QueueService }

func NewQueueHandler(svc service.QueueService) *QueueHandler { return &QueueHandler{svc: svc} }

type walkInRequest struct {
	DepartmentID int64 `json:"department_id" validate:"required,gt=0"`
}

type staffWalkInRequest struct {
	DepartmentID int64  `json:"department_id" validate:"required,gt=0"`
	PatientID    string `json:"patient_id" validate:"omitempty,uuid"`
	DisplayName  string `json:"display_name" validate:"max=100"`
}

type walkInTicketDTO struct {
	QueueEntryID uuid.UUID `json:"queue_entry_id"`
	TicketNo     int       `json:"ticket_no"`
	AheadCount   int       `json:"ahead_count"`
}

type queueStateDTO struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

// staffQueueEntryDTO は氏名・症状メモを含む。スタッフ向け以外で使わない。
type staffQueueEntryDTO struct {
	ID          uuid.UUID `json:"id"`
	TicketNo    int       `json:"ticket_no"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	PatientName string    `json:"patient_name"`
	SymptomNote string    `json:"symptom_note,omitempty"`
	CheckedInAt time.Time `json:"checked_in_at"`
}

func queueError(c echo.Context, err error) error {
	var it *service.InvalidTransitionError
	var wc *service.WalkInClosedError
	switch {
	case errors.Is(err, service.ErrQueueEntryNotFound), errors.Is(err, service.ErrAppointmentNotFound):
		return apierror.Write(c, http.StatusNotFound, "not_found", "not found")
	case errors.As(err, &it):
		return apierror.Write(c, http.StatusConflict, "invalid_transition", fmt.Sprintf("cannot perform this operation in the current state (%s)", it.Current))
	case errors.Is(err, service.ErrAlreadyInQueue):
		return apierror.Write(c, http.StatusConflict, "already_in_queue", "already in the queue")
	case errors.As(err, &wc):
		msg := "walk-in registration is closed"
		switch wc.Reason {
		case "after_cutoff":
			msg = "walk-in registration is closed (after the cutoff time)"
		case "capacity_reached":
			msg = "walk-in registration is closed (capacity reached)"
		}
		return apierror.Write(c, http.StatusUnprocessableEntity, "walkin_closed", msg)
	}
	return serviceError(c, err)
}

func toTicketDTO(t *service.WalkInTicket) walkInTicketDTO {
	return walkInTicketDTO{QueueEntryID: t.QueueEntryID, TicketNo: t.TicketNo, AheadCount: t.AheadCount}
}

// CreateWalkIn: POST /api/v1/walk-ins（患者本人）
func (h *QueueHandler) CreateWalkIn(c echo.Context) error {
	pid, ok := patientID(c)
	if !ok {
		return apierror.Write(c, http.StatusUnauthorized, "unauthorized", "")
	}
	var req walkInRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	t, err := h.svc.CreateWalkIn(c.Request().Context(), service.WalkInInput{DepartmentID: req.DepartmentID, PatientID: &pid})
	if err != nil {
		return queueError(c, err)
	}
	return c.JSON(http.StatusCreated, toTicketDTO(t))
}

// CreateStaffWalkIn: POST /api/v1/staff/walk-ins（スタッフ代行。アカウントが無い患者も可）
func (h *QueueHandler) CreateStaffWalkIn(c echo.Context) error {
	var req staffWalkInRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	in := service.WalkInInput{DepartmentID: req.DepartmentID, DisplayName: req.DisplayName}
	if req.PatientID != "" {
		id, _ := uuid.Parse(req.PatientID) // validate:"uuid" で検証済み
		in.PatientID = &id
	}
	t, err := h.svc.CreateWalkIn(c.Request().Context(), in)
	if err != nil {
		return queueError(c, err)
	}
	return c.JSON(http.StatusCreated, toTicketDTO(t))
}

// action はパスの ID を読んで操作を実行する共通処理。不正な ID は存在しない ID と同じ 404。
func (h *QueueHandler) action(do func(h *QueueHandler, c echo.Context, id uuid.UUID) (*service.QueueState, error)) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, ok := parseID(c)
		if !ok {
			return apierror.Write(c, http.StatusNotFound, "not_found", "not found")
		}
		st, err := do(h, c, id)
		if err != nil {
			return queueError(c, err)
		}
		return c.JSON(http.StatusOK, queueStateDTO{ID: st.ID, Status: st.Status})
	}
}

// ListStaffQueue: GET /api/v1/staff/queue?department_id=
func (h *QueueHandler) ListStaffQueue(c echo.Context) error {
	deptID, _, bad := queryInt64(c, "department_id", true)
	if bad != nil {
		return validationFailed(c, *bad)
	}
	rows, err := h.svc.ListStaffQueue(c.Request().Context(), deptID)
	if err != nil {
		return queueError(c, err)
	}
	out := make([]staffQueueEntryDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, staffQueueEntryDTO{ID: r.ID, TicketNo: r.TicketNo, Source: r.Source, Status: r.Status, PatientName: r.PatientName, SymptomNote: r.SymptomNote, CheckedInAt: r.CheckedInAt})
	}
	return c.JSON(http.StatusOK, out)
}

// PatientRoutes は患者本人の当日受付（認証必須）。
func (h *QueueHandler) PatientRoutes(g *echo.Group, jwtAuth echo.MiddlewareFunc) {
	g.POST("/walk-ins", h.CreateWalkIn, jwtAuth, noStore)
}

// StaffRoutes は /staff 配下。jwtAuth → staffOnly の順に適用し、待機列の参照は audit で記録する。
func (h *QueueHandler) StaffRoutes(g *echo.Group, jwtAuth, staffOnly, audit echo.MiddlewareFunc) {
	g.POST("/walk-ins", h.CreateStaffWalkIn, jwtAuth, staffOnly, noStore)
	// 監査ログは「個人情報を返す」リクエストだけ記録するため、パラメータ検証を監査の前に行う
	g.GET("/queue", h.ListStaffQueue, jwtAuth, staffOnly, requirePositiveIntQuery("department_id"), audit, noStore)
	g.POST("/queue/:id/check-in", h.action(func(h *QueueHandler, c echo.Context, id uuid.UUID) (*service.QueueState, error) {
		return h.svc.CheckIn(c.Request().Context(), id)
	}), jwtAuth, staffOnly, noStore)
	g.POST("/queue/:id/call", h.action(func(h *QueueHandler, c echo.Context, id uuid.UUID) (*service.QueueState, error) {
		return h.svc.Call(c.Request().Context(), id)
	}), jwtAuth, staffOnly, noStore)
	g.POST("/queue/:id/start", h.action(func(h *QueueHandler, c echo.Context, id uuid.UUID) (*service.QueueState, error) {
		return h.svc.Start(c.Request().Context(), id)
	}), jwtAuth, staffOnly, noStore)
	g.POST("/queue/:id/complete", h.action(func(h *QueueHandler, c echo.Context, id uuid.UUID) (*service.QueueState, error) {
		return h.svc.Complete(c.Request().Context(), id)
	}), jwtAuth, staffOnly, noStore)
}

// requirePositiveIntQuery は必須の正整数クエリパラメータを検証するミドルウェア。不正なら 422。
func requirePositiveIntQuery(name string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, _, bad := queryInt64(c, name, true); bad != nil {
				return validationFailed(c, *bad)
			}
			return next(c)
		}
	}
}
