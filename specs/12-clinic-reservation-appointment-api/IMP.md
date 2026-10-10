---
kairo:
  node_id: "imp:12-clinic-reservation-appointment-api"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "12-clinic-reservation-appointment-api"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T17:38:15+09:00"
  updated_at: "2026-10-10T17:38:15+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:12-clinic-reservation-appointment-api"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/appointment_service.go', 'backend/internal/domain/notification.go', 'backend/internal/handler/appointment_handler.go', 'backend/db/query/slot.sql,', 'backend/internal/repository/errors.go']
  band: "Green"
  last_validated: "2026-10-10T17:38:15+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 12
---

# IMP: 予約 API（確定・一覧・変更・キャンセル）

| 項目 | 値 |
|---|---|
| imp_id | IMP-12-clinic-reservation-appointment-api |
| imp_version | 1.0.0 |
| source_issue | #12（https://github.com/kava2108/booking/issues/12） |
| task | clinic-reservation tasks.md §2.2 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> 予約の確定・一覧・日時変更・キャンセルを実装する。枠確保と予約作成を 1 トランザクションで行い、定員超過・二重予約を防ぐ。
> 確認/キャンセル/変更の通知は同一 Tx でアウトボックスに登録し、本文に症状メモを含めない。
> 患者向けクエリは常に patient_id で絞り、他人の予約は 404 とする。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 予約の中核。🔵 同時リクエストで定員を超える／枠だけ確保されて予約が作られない状態を作ってはならない。

### 1.2 目標
20 並行でも定員を超えず、失敗時は枠確保が巻き戻り、キャンセル・変更で枠が正しく戻る。

### 1.3 非目標（Out of Scope）
通知の送信（#14）、来院受付と待機列（#13）。

---

## 2. スニペット（変更対象の既存コード）

#5 の TxRunner / Repositories、SlotRepository.ReserveSlot（満席で pgx.ErrNoRows）、AppointmentRepository（患者スコープ）。#10 の Identity。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/appointment_service.go | 追加 | Create / List / Cancel / Reschedule。受付期間・キャンセル期限・重複の検証 | #13, #14 |
| backend/internal/domain/notification.go | 追加 | 通知種別と payload（症状メモを含めない） | #14 |
| backend/internal/handler/appointment_handler.go | 追加 | POST /appointments、GET /appointments/me、PATCH/DELETE /appointments/{id} | — |
| backend/db/query/slot.sql, appointment.sql | 修正 | GetSlotDetail（診療科名・医師名つき）を追加 | — |
| backend/internal/repository/errors.go | 修正 | 一意制約名の判定ヘルパー | — |

### 3.2 API 変更

POST /api/v1/appointments、GET /api/v1/appointments/me、PATCH|DELETE /api/v1/appointments/{id}（実装）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### POST /api/v1/appointments

**入力**
```
入力: {"slot_id","visit_type":"revisit","symptom_note":"咳"}（Bearer 必須）
```

**出力**
```
出力: 201 {id, appointment_no, slot_start, department, doctor}
```

**エラー**
満席 409 slot_full／同時刻に予約あり 409 duplicate_appointment／過去・休診・受付期間外 422／入力不正 422

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
| Tx 内での外部 I/O（メール送信） | ロック保持・失敗連鎖 | アウトボックスに登録のみ |
| 通知 payload への symptom_note | 要配慮情報の拡散 | 予約番号・日時・診療科・医師のみ |
| 患者 ID を指定せず ID だけで取得するクエリの患者向け利用 | IDOR | GetAppointment(id, patient_id) |

---

## 6. タスク詳細

### TASK-0001: 予約 API（確定・一覧・変更・キャンセル）

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-002-AC-1: WHEN ログイン済み患者が空き枠・来院区分（初診/再診）・症状メモを送信する THEN システムは予約を確定し予約番号を発行する（Must）
- [WHEN/IF] REQ-002-AC-2: WHEN 同一枠に複数の予約が同時に送信された THEN システムは排他制御により定員を超える予約を確定せず、後続に HTTP 409 を返す（Must）
- [WHEN/IF] REQ-002-AC-4: 予約確定の応答は 3 秒以内に返す（Must）
- [WHEN/IF] REQ-002-AC-5: WHEN 患者が同一時間帯に既に別の予約を持つ THEN システムは重複予約を拒否する（Should）
- [WHEN/IF] REQ-003-AC-1: WHEN 患者がマイページを開く THEN システムは自分の予約一覧（今後・過去）を表示する（Must）
- [WHEN/IF] REQ-003-AC-2: WHEN 患者がキャンセル期限内に予約をキャンセルする THEN システムは枠を解放し通知を送信する（Must）
- [WHEN/IF] REQ-003-AC-3: WHEN キャンセル期限を過ぎている THEN システムはキャンセル不可とし、医療機関への連絡方法を案内する（Should）
- [WHEN/IF] REQ-003-AC-4: WHEN 患者が日時を変更する THEN システムは新枠の確保と旧枠の解放を原子的に行う（Should）
- [WHEN/IF] REQ-007-AC-2: 症状メモ等の要配慮個人情報は、本人と権限を持つスタッフ以外が参照できないようアクセス制御する（Must）

**実装手順**
1. domain/notification.go（種別・payload）
2. Create: Tx{ 枠取得・検証 → 同時刻重複 → ReserveSlot → CreateAppointment（予約番号の衝突は再試行）→ 通知登録 }
3. List: 本人分 + cancelable 判定
4. Cancel: 期限判定 → Tx{ Cancel → ReleaseSlot → 通知 }
5. Reschedule: Tx{ 新枠 Reserve → 付替え → 旧枠 Release → 通知 }
6. ハンドラー・結線・統合テスト

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 予約確定で booked_count が 1 増え、通知が 1 件登録される |
| 異常系 | 満席 409／重複 409／過去枠・休診枠・受付期間外 422／期限超過のキャンセル 422／他人の予約 404 |
| 境界値 | キャンセル期限ちょうど（可/不可の境界）／capacity=1 に 20 並行で成功 1 件 |

**受け入れ基準チェックリスト**
- [ ] 正常系: 予約確定・一覧・変更・キャンセル
- [ ] 並行: 20 並行で定員超過なし、失敗側は枠が増えない
- [ ] 異常系: 満席/重複/過去/休診/期間外/期限超過
- [ ] 境界値: キャンセル期限ちょうど
- [ ] セキュリティ: 他人の予約の取得・変更・キャンセルは 404
- [ ] セキュリティ: 通知 payload に症状メモが無い
- [ ] 正常系: 変更で旧枠が解放され新枠が確保される（原子的）
- [ ] 応答時間: 確定は 3 秒以内（テストで計測）

**依存関係**
- 前提: #11（枠）、#5, #6, #10
- 後続: #13, #14

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
| Integration | 並行予約・原子性・IDOR | pgtest + httptest |
| E2E | P4 | Playwright |

### 8.2 テスト観点

- [ ] 正常系: 予約確定・一覧・変更・キャンセル
- [ ] 並行: 20 並行で定員超過なし、失敗側は枠が増えない
- [ ] 異常系: 満席/重複/過去/休診/期間外/期限超過
- [ ] 境界値: キャンセル期限ちょうど
- [ ] セキュリティ: 他人の予約の取得・変更・キャンセルは 404
- [ ] セキュリティ: 通知 payload に症状メモが無い
- [ ] 正常系: 変更で旧枠が解放され新枠が確保される（原子的）
- [ ] 応答時間: 確定は 3 秒以内（テストで計測）

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
二重予約や枠カウントのずれが見つかった場合。

### 10.2 ロールバック手順
予約 API ルートを無効化。slots.booked_count を appointments の booked 件数から再計算する SQL を用意。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| 予約番号の衝突でリトライが増える | L | M | 4 桁ランダム + 日付、衝突時は Tx ごと最大 5 回再試行 |
| 変更時に新旧枠の両方を保持してしまう | H | L | Tx 内で付替え→旧解放。失敗テストで検証 |
| キャンセル期限の基準時刻（サーバー時計）のずれ | M | L | 時刻注入で境界テスト、NTP 前提 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] Tx 境界が service に閉じ、handler が Tx を知らない設計か

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
