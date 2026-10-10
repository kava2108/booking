---
kairo:
  node_id: "imp:13-clinic-reservation-queue-staff-walkin"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "13-clinic-reservation-queue-staff-walkin"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T17:38:15+09:00"
  updated_at: "2026-10-10T17:38:15+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:13-clinic-reservation-queue-staff-walkin"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/queue_service.go', 'backend/internal/handler/queue_handler.go', 'backend/db/query/appointment.sql,', 'backend/internal/app/app.go']
  band: "Green"
  last_validated: "2026-10-10T17:38:15+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 13
---

# IMP: 待機列とスタッフ操作・当日受付

| 項目 | 値 |
|---|---|
| imp_id | IMP-13-clinic-reservation-queue-staff-walkin |
| imp_version | 1.0.0 |
| source_issue | #13（https://github.com/kava2108/booking/issues/13） |
| task | clinic-reservation tasks.md §2.3 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> 来院受付・呼出・診察開始/完了・当日受付（患者本人/スタッフ代行）と、スタッフ向け待機列の参照を実装する。
> 状態遷移は SQL の条件付き UPDATE で保証し、不正遷移は 409、存在しなければ 404。
> 待機列の個人情報参照は StaffOnly + 監査ログ。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 待ち時間の元データ（来院・呼出・完了の記録）が無い。🔵 予約患者と当日受付を同じ列で扱い、順序を決める必要がある（遅刻者が先着を追い越さない）。

### 1.2 目標
受付〜完了を 1 タップ相当の API で記録でき、待機列と当日受付の制限（締切・定員）が働く。

### 1.3 非目標（Out of Scope）
待ち時間の算出と公開（P2 3.1）、休診設定（P2 3.2）、画面。

---

## 2. スニペット（変更対象の既存コード）

#5 の QueueRepository（NextTicketNo, EnqueueEntry, 遷移 4 種, CountAhead, ListStaffQueue）。#6 の StaffOnly / AuditLog。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/queue_service.go | 追加 | CheckIn / Call / Start / Complete / CreateWalkIn（締切・定員・重複）/ ListStaffQueue | P2 3.1 |
| backend/internal/handler/queue_handler.go | 追加 | POST /walk-ins、/staff/walk-ins、/staff/queue/{id}/{check-in,call,start,complete}、GET /staff/queue | — |
| backend/db/query/appointment.sql, queue.sql | 修正 | GetAppointmentWithSlot, CompleteAppointment, CountWalkInsForDate, CountActiveQueueEntriesForPatient を追加 | — |
| backend/internal/app/app.go | 修正 | staff/patient ルートの結線（StaffOnly, AuditLog） | — |

### 3.2 API 変更

POST /api/v1/walk-ins、POST /api/v1/staff/walk-ins、POST /api/v1/staff/queue/{id}/{check-in|call|start|complete}、GET /api/v1/staff/queue（実装）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### POST /api/v1/staff/queue/{id}/check-in

**入力**
```
入力: id = 予約 ID（Bearer: staff）
```

**出力**
```
出力: 200 {id: queue_entry_id, status:"waiting"}
```

**エラー**
予約なし 404／キャンセル済み・当日でない 422／二重受付 409

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| 既存のみ | — | — |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| interface{} / any の無制限使用 | 型安全性 | 具体型・ジェネリクス |
| panic の業務ロジック使用 | Steering 禁止 | error を返す |
| 状態遷移を SELECT → UPDATE の 2 段階で実装 | 競合で二重遷移 | WHERE status=現在値 の単一 UPDATE |
| 公開 API・患者向け応答に氏名/症状メモを含める | 要配慮情報 | StaffQueueEntry のみ |

---

## 6. タスク詳細

### TASK-0001: 待機列とスタッフ操作・当日受付

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-005-AC-1: WHEN スタッフが患者の来院受付を記録する THEN システムはその患者を待機列に追加する（Must）
- [WHEN/IF] REQ-005-AC-2: WHEN スタッフが呼出・診察開始・診察完了を記録する THEN システムは待機列と診察実績を更新し、待ち時間表示へ反映する（Must）
- [WHEN/IF] REQ-005-AC-4: スタッフ機能はロールベースで制限し、患者は利用できない（Must）
- [WHEN/IF] REQ-008-AC-1: WHEN 患者またはスタッフが当日受付を登録する THEN システムは診療科の待機列の末尾に追加し受付番号を発行する（Must）
- [WHEN/IF] REQ-008-AC-2: 待機列は予約患者の予約時刻と当日受付患者の受付順を合わせた順序で管理し、待ち時間の算出に含める（Must）
- [WHEN/IF] REQ-008-AC-3: IF 受付終了時刻を過ぎている・定員に達した THEN システムは当日受付を拒否し理由を表示する（Must）
- [WHEN/IF] REQ-007-AC-2: 症状メモ等の要配慮個人情報は、本人と権限を持つスタッフ以外が参照できないようアクセス制御する（Must）

**実装手順**
1. CheckIn: 予約取得 → 検証（booked・当日）→ 採番（衝突時再試行）→ sort_time = max(予約時刻, 受付時刻)
2. Call/Start/Complete: 遷移 → ErrNoRows なら存在確認で 404/409。Complete は予約を completed に（同一 Tx）
3. CreateWalkIn: 診療科・締切時刻・定員・患者の重複を検証 → 採番 → ahead_count
4. ListStaffQueue + AuditLog
5. ハンドラー・結線・統合テスト

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 予約者の受付 → 呼出 → 開始 → 完了、予約は completed |
| 異常系 | 不正遷移 409／存在しない 404／締切後・定員超過の当日受付 422／二重の当日受付 409 |
| 境界値 | 締切時刻ちょうど（拒否）／定員ちょうど（最後の 1 人は可）／遅刻者は先着の当日患者を追い越さない |

**受け入れ基準チェックリスト**
- [ ] 正常系: 来院〜完了、sort_time の規則
- [ ] 並行: 同時の当日受付で番号が重複しない（衝突時再試行）
- [ ] 異常系: 不正遷移 409、不明 404
- [ ] 境界値: 締切ちょうど／定員ちょうど
- [ ] セキュリティ: patient は staff API で 403、staff 一覧は監査ログに記録
- [ ] セキュリティ: 患者向け応答に他人の情報が無い
- [ ] 正常系: スタッフ代行の無アカウント受付（display_name）

**依存関係**
- 前提: #11, #12, #5, #6
- 後続: P2 3.1, 3.2

---

## 7. 禁止事項

- [ ] 本 Issue の「3.1 変更ファイル一覧」以外のファイルを変更しないこと
- [ ] 他 Issue が所有するファイル・マイグレーション番号を変更しないこと
- [ ] 既存のテストを削除・修正しないこと
- [ ] 新たな外部依存を追加しないこと（5.1 に無いもの。必要なら IMP を更新）
- [ ] 禁止パターン（5.2）を使わないこと

---

## 8. テスト戦略

### 8.1 テスト方針

| レベル | カバレッジ目標 | 使用フレームワーク |
|---|---|---|
| Unit | 90% | testify + mockery |
| Integration | 遷移・採番並行・権限・監査 | pgtest + httptest |
| E2E | P4 | Playwright |

### 8.2 テスト観点

- [ ] 正常系: 来院〜完了、sort_time の規則
- [ ] 並行: 同時の当日受付で番号が重複しない（衝突時再試行）
- [ ] 異常系: 不正遷移 409、不明 404
- [ ] 境界値: 締切ちょうど／定員ちょうど
- [ ] セキュリティ: patient は staff API で 403、staff 一覧は監査ログに記録
- [ ] セキュリティ: 患者向け応答に他人の情報が無い
- [ ] 正常系: スタッフ代行の無アカウント受付（display_name）

### 8.3 テスト除外事項
生成物（sqlc 出力・mockery 出力）は直接テストしない。

### 8.4 テストデータ方針
個人情報を含む実データは使用しない。テスト用の固定 UUID・ダミー氏名のみ。

---

## 9. Definition of Done

- [ ] 全テストケースが通過している
- [ ] 型エラー・ビルドエラーが 0 件である
- [ ] 既存テストが破壊されていない
- [ ] 禁止事項（セクション 7）を全て遵守している
- [ ] IMP の受け入れ基準（EARS）を全て満たしている
- [ ] コードレビュー指摘事項が解消されている

---

## 10. ロールバック計画

### 10.1 ロールバックトリガー条件
待機列の順序や状態が壊れた場合。

### 10.2 ロールバック手順
当日の queue_entries を手動で補正（status 更新のみ）。ルートを無効化。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| スタッフの操作ミス（誤って完了） | M | M | キャンセル・巻き戻しは未実装→TODO。監査で追跡 |
| 遅刻者の sort_time 規則が現場の運用と合わない | M | M | 規則を service の 1 箇所に集約し変更容易に |
| 採番の競合再試行が増える | L | L | 最大 5 回。UNIQUE が最終防壁 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] 状態遷移のルールが SQL（WHERE status）に集約され、service に二重定義されていないか

---

## 13. 承認記録

| ロール | 担当者 | 承認日時 | コメント |
|---|---|---|---|
| アーキテクト | | | |
| 実装者 | | | |

---

## Appendix: 変更履歴

| バージョン | 変更内容 | 変更者 | 日時 |
|---|---|---|---|
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T17:38:15+09:00 |
