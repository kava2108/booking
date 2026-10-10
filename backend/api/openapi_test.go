package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IMP 6-clinic-reservation-rbac-audit-openapi TASK-0001: OpenAPI 定義の検証。
// フロントの型はこの定義から生成する（手書き禁止）ため、契約として厳密にテストする。

func load(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile("openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))
	return doc
}

type op struct {
	method, path string
	item         *openapi3.Operation
}

func operations(doc *openapi3.T) []op {
	var out []op
	for path, item := range doc.Paths.Map() {
		for method, o := range item.Operations() {
			out = append(out, op{method, path, o})
		}
	}
	return out
}

// 設計書 §API Design の 19 エンドポイントが過不足なく定義されている
func TestOpenAPI_EndpointsMatchDesign(t *testing.T) {
	want := []string{
		"GET /api/departments", "GET /api/doctors", "GET /api/slots",
		"POST /api/appointments", "GET /api/appointments/me",
		"PATCH /api/appointments/{id}", "DELETE /api/appointments/{id}",
		"GET /api/wait-times", "GET /api/appointments/{id}/position",
		"POST /api/walk-ins", "POST /api/staff/walk-ins",
		"POST /api/staff/queue/{id}/check-in", "POST /api/staff/queue/{id}/call",
		"POST /api/staff/queue/{id}/start", "POST /api/staff/queue/{id}/complete",
		"GET /api/staff/queue", "POST /api/staff/closures",
		"PUT /api/admin/schedules", "PUT /api/admin/settings",
	}
	var got []string
	for _, o := range operations(load(t)) {
		got = append(got, o.method+" "+o.path)
	}
	assert.ElementsMatch(t, want, got)
}

func TestOpenAPI_OperationIDsUniqueAndPresent(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range operations(load(t)) {
		assert.NotEmpty(t, o.item.OperationID, "%s %s", o.method, o.path)
		assert.False(t, seen[o.item.OperationID], "operationId 重複: %s", o.item.OperationID)
		seen[o.item.OperationID] = true
	}
}

func roles(o op) []string {
	raw, ok := o.item.Extensions["x-required-roles"]
	if !ok {
		return nil
	}
	var out []string
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return out
}

func hasSecurity(doc *openapi3.T, o op) bool {
	sec := o.item.Security
	if sec == nil {
		sec = &doc.Security
	}
	return sec != nil && len(*sec) > 0
}

// REQ-005-AC-4: 認可要件が定義に明記されている（実装の権限マトリクスのテスト基準になる）
func TestOpenAPI_AuthorizationMatrix(t *testing.T) {
	doc := load(t)
	public := map[string]bool{"/api/departments": true, "/api/doctors": true, "/api/slots": true, "/api/wait-times": true}

	for _, o := range operations(doc) {
		name := o.method + " " + o.path
		switch {
		case public[o.path]:
			assert.False(t, hasSecurity(doc, o), "%s は認証不要", name)
		case strings.HasPrefix(o.path, "/api/staff/"):
			assert.True(t, hasSecurity(doc, o), name)
			assert.ElementsMatch(t, []string{"staff", "clinic_admin"}, roles(o), name)
		case strings.HasPrefix(o.path, "/api/admin/"):
			assert.True(t, hasSecurity(doc, o), name)
			assert.ElementsMatch(t, []string{"clinic_admin"}, roles(o), name)
		default:
			assert.True(t, hasSecurity(doc, o), "%s は患者トークン必須", name)
			assert.Empty(t, roles(o), name)
		}
	}
}

// REQ-007-AC-1: 公開 API の応答に個人を特定できる項目を含めない
func TestOpenAPI_PublicWaitTimesHasNoPII(t *testing.T) {
	doc := load(t)
	resp := doc.Paths.Find("/api/wait-times").Get.Responses.Status(200).Value.Content.Get("application/json").Schema
	banned := []string{"name", "patient", "symptom", "display_name", "email", "ticket_no", "phone", "user"}

	var walk func(s *openapi3.SchemaRef)
	walk = func(s *openapi3.SchemaRef) {
		if s == nil || s.Value == nil {
			return
		}
		for prop, child := range s.Value.Properties {
			if prop == "name" && s.Value.Properties["department_id"] != nil {
				continue // 診療科名は公開情報
			}
			for _, b := range banned {
				assert.NotContains(t, strings.ToLower(prop), b, "公開 API に %q を含めない", prop)
			}
			walk(child)
		}
		walk(s.Value.Items)
	}
	walk(resp)
}

// 待ち時間の応答は設計書どおり（basis / status / reason / note / updated_at）
func TestOpenAPI_WaitTimesSchema(t *testing.T) {
	doc := load(t)
	schema := doc.Paths.Find("/api/wait-times").Get.Responses.Status(200).Value.Content.Get("application/json").Schema.Value
	for _, k := range []string{"updated_at", "note", "departments"} {
		assert.Contains(t, schema.Properties, k)
	}
	item := schema.Properties["departments"].Value.Items.Value
	for _, k := range []string{"department_id", "name", "status", "waiting_count", "wait_minutes", "basis", "reason"} {
		assert.Contains(t, item.Properties, k)
	}
	assert.ElementsMatch(t, []any{"available", "unavailable"}, item.Properties["status"].Value.Enum)
	assert.ElementsMatch(t, []any{"actual", "standard"}, item.Properties["basis"].Value.Enum)
}

func TestOpenAPI_ErrorResponses(t *testing.T) {
	doc := load(t)
	has := func(method, path string, code int) bool {
		item := doc.Paths.Find(path)
		o := item.GetOperation(method)
		return o != nil && o.Responses.Status(code) != nil
	}
	assert.True(t, has("POST", "/api/appointments", 409), "満席 409")
	assert.True(t, has("POST", "/api/appointments", 422), "入力不正/重複 422")
	assert.True(t, has("DELETE", "/api/appointments/{id}", 422), "キャンセル期限超過 422")
	assert.True(t, has("POST", "/api/walk-ins", 422), "当日受付の締切 422")
	assert.True(t, has("GET", "/api/wait-times", 429), "公開 API のレートリミット 429")
	assert.True(t, has("POST", "/api/staff/queue/{id}/call", 409), "不正遷移 409")
	assert.True(t, has("GET", "/api/staff/queue", 403), "権限不足 403")
	assert.True(t, has("GET", "/api/appointments/me", 401), "未認証 401")
}

// 症状メモは本人・スタッフ向けスキーマにのみ存在する
func TestOpenAPI_SymptomNoteOnlyInPrivateSchemas(t *testing.T) {
	doc := load(t)
	for name, s := range doc.Components.Schemas {
		if _, has := s.Value.Properties["symptom_note"]; has {
			assert.Contains(t, []string{"CreateAppointmentRequest", "AppointmentSummary", "StaffQueueEntry"}, name,
				"symptom_note を含むスキーマ %s は想定外", name)
		}
	}
	assert.NotContains(t, doc.Components.Schemas["WaitTimesResponse"].Value.Properties, "symptom_note")
}

// 設計書の例: 予約確定のエラー体と必須項目
func TestOpenAPI_CreateAppointmentRequestRequiredFields(t *testing.T) {
	doc := load(t)
	req := doc.Components.Schemas["CreateAppointmentRequest"].Value
	assert.ElementsMatch(t, []string{"slot_id", "visit_type"}, req.Required)
	assert.ElementsMatch(t, []any{"first", "revisit"}, req.Properties["visit_type"].Value.Enum)
}
