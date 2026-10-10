# Issue 構造定義: 6-clinic-reservation-rbac-audit-openapi

## 概要
ロール認可・監査ログ・OpenAPI 定義

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#1.3
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-005-AC-4 | スタッフ機能はロールベースで制限し、患者は利用できない | Must |
| REQ-007-AC-2 | 症状メモ等の要配慮個人情報は、本人と権限を持つスタッフ以外が参照できないようアクセス制御する | Must |
| REQ-007-AC-3 | 通信は TLS で暗号化し、個人情報へのアクセスを監査ログに記録する | Must |

## タスク一覧
- JWT クレームの role を検証する Echo ミドルウェア（staff / clinic_admin）
- 個人情報アクセスの監査ログ記録ミドルウェア
- 全 API の OpenAPI 定義を作成し openapi-typescript で型生成を設定
- 公開 API 用レートリミットを設定（ulule/limiter）

## 依存関係
- 先行タスク: なし（前提: user-auth 完了）
- 並列波形: P0

## GitHub Issue
- Issue 番号: #6
- Issue URL: https://github.com/kava2108/booking/issues/6
