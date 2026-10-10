# Red フェーズ: 6-clinic-reservation-rbac-audit-openapi / 1.3

実装前に `go vet ./...` → Role / SetIdentity / RequireRole / AuditLog / openapi.yaml が無く失敗（期待どおり）。

| テスト | ケース | AC |
|---|---|---|
| domain/role_test | 4 値のみ Valid、旧値（user, facility_admin）は不正 | — |
| middleware/role_test | **StaffOnly 権限マトリクス**（patient 403 / staff・clinic_admin 通過 / system_admin 403 / 未知・空 403）、AdminOnly マトリクス、未認証 401、403 本文、引数なしは全拒否、拒否時にハンドラー非実行 | REQ-005-AC-4 |
| middleware/audit_test | actor/action/target の記録（ハンドラー前）、クエリ文字列・ボディ非記録、**監査書込み失敗でフェイルクローズ（500・ハンドラー非実行）**、未認証 401、target 関数 | REQ-007-AC-3 |
| config_test | 公開 API 用レートリミットの既定値（60 / 1m）・上書き・不正値 | — |
| api/openapi_test | 設計書の 19 エンドポイントと一致、operationId 一意、**認可マトリクス（x-required-roles と security）**、**公開 wait-times に個人情報項目が無い**、待ち時間スキーマ、エラー応答（409/422/429/403/401）、symptom_note は限定スキーマのみ、必須項目 | REQ-005-AC-4, REQ-007-AC-1/2 |
| フロント型 | gen:api 成功 + tsc strict、設計書の応答例が型に収まる、公開型に個人情報キーが無い、不正な visit_type はコンパイルエラー | REQ-007-AC-1 |

変異確認: OpenAPI に patient_name を足す / staff API の x-required-roles を欠落させる → それぞれテストが失敗することを確認し、元に戻した。
