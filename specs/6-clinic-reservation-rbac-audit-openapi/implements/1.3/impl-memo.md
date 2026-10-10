---
issue_id: 6-clinic-reservation-rbac-audit-openapi
task_id: 1.3
created_at: 2026-10-10T16:55:06+09:00
---

# 実装判断メモ: 6-clinic-reservation-rbac-audit-openapi / 1.3

## 採用したアプローチ

- **権限マトリクスの確定**: staff API = staff + clinic_admin、admin API = clinic_admin のみ。**system_admin はどちらも不可**（プラットフォーム運用者が患者の氏名・症状メモを見られないようにするため）。未知・空のロールも拒否。要件にない判断なので、変更したい場合は IMP を更新すること。
- **Identity をミドルウェア間の契約にした**: jwt_auth（user-auth P1）は検証後に `SetIdentity` を呼ぶだけでよい。テストは JWT 無しで任意ロールを注入でき、IMP のリスク（jwt_auth 未完で #6 のテストが書けない）を解消した。
- **Role は domain/role.go**: user-auth の domain/user.go と衝突しないよう分離し、user-auth の tasks.md に「再定義しない」と追記。値は migration 000001 の CHECK と一致。
- **監査ログはハンドラーの前に書き、失敗したら 500（フェイルクローズ）**: 個人情報を返した後に監査が欠落する状態を作らないため。DB 障害時はスタッフ画面も参照不可になる点がトレードオフ。記録は actor / action / target（パスのみ）。クエリ文字列とボディは記録しない。
- **未認証は 401、権限不足は 403**: RequireRole 単体でも 401 を返す（JWT ミドルウェアを付け忘れても通さない）。
- **OpenAPI 3.0.3 + 契約テスト**: x-required-roles を拡張として持たせ、実装の権限マトリクスとテストで突き合わせる。公開 wait-times に個人情報項目が無いこと、symptom_note が限定スキーマにしか無いことを機械的に検証。変異（patient_name 追加 / ロール欠落）で検出を確認。
- **他人のリソースは 404**: IDOR で存在有無を漏らさない。OpenAPI の NotFound にも明記。
- **フロントは型生成の足場のみ**: React / Vite のスキャフォールドは別タスク（3.3 など）。ここでは package.json / tsconfig / 生成物 / 型レベル契約チェック。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| system_admin も staff API を許可 | 要配慮個人情報の最小権限に反する |
| 監査を非同期（ハンドラー後）で書く | 失敗時に記録欠落のまま個人情報が返る |
| oapi-codegen で Go 側も生成 | 現段階では過剰。Echo ハンドラーは手書きし、契約テストで担保 |
| JWT クレームを直接 context key で受け渡し | Identity 型に集約して型安全に |

## 既存コードへの影響

#3 の config に公開 API 用レート設定を追加（既存キーは変更なし）。#3 の rate_limiter は変更なし（別 Rate で再利用）。

## TODO・未解決事項

- [ ] user-auth P1: jwt_auth が検証後に SetIdentity を呼ぶ
- [ ] ルーティング: /api/v1/staff/** に StaffOnly、/api/v1/admin/** に AdminOnly、PII を返すハンドラーに AuditLog、公開 API に NewRateLimiter
- [ ] 実装時に OpenAPI と応答を突き合わせる契約テストをハンドラー側にも追加（乖離リスク）
- [ ] clinic_admin の割り当て手段（初期ユーザーの作り方）が未定義
- [ ] Rate limit の Retry-After が OpenAPI に定義済みだが、公開 API 用の組込みは未


## 変更 (2026-10-10): API パスを /api/v1 に統一

user-auth（/api/v1/auth/*）に合わせ、clinic-reservation の全エンドポイントを `/api/v1/*` に変更。OpenAPI、契約テスト、生成型（frontend/src/api/schema.d.ts）、設計書、タスク、steering の命名規約を更新。互換性を壊す変更のときだけ /api/v2 を追加する。
