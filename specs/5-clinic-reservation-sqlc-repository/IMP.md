---
kairo:
  node_id: "imp:5-clinic-reservation-sqlc-repository"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "5-clinic-reservation-sqlc-repository"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T16:13:39+09:00"
  updated_at: "2026-10-10T16:13:39+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:5-clinic-reservation-sqlc-repository"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/db/query/slot.sql', 'backend/db/query/appointment.sql', 'backend/db/query/queue.sql', 'backend/db/query/notification.sql', 'backend/db/query/admin.sql', 'backend/internal/repository/*_repository.go', 'backend/internal/repository/mocks/*']
  band: "Green"
  last_validated: "2026-10-10T16:13:39+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 5
---

# IMP: sqlc クエリと repository interface（clinic-reservation）

| 項目 | 値 |
|---|---|
| imp_id | IMP-5-clinic-reservation-sqlc-repository |
| imp_version | 1.0.0 |
| source_issue | #5（https://github.com/kava2108/booking/issues/5） |
| task | clinic-reservation tasks.md §1.2 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> clinic-reservation のクエリを sqlc で定義し、service が依存する repository interface と mockery モックを用意する。
> 枠の条件付き UPDATE（排他確保）、待機列の集計、通知の SKIP LOCKED 取得を含む。
> #2 で作る sqlc 設定に相乗りする。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 P1 の4タスク（2.1〜2.4）が repository interface を前提とするため、先に契約を固定する必要がある。

### 1.2 目標
`sqlc generate` が通り、主要 interface のモックが生成される。条件付き UPDATE が並行実行でも定員を超えない。

### 1.3 非目標（Out of Scope）
service 実装、HTTP ハンドラー、user-auth 用クエリ。

---

## 2. スニペット（変更対象の既存コード）

既存コードなし。#2 の sqlc.yaml と #4 のスキーマに依存。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/db/query/slot.sql | 追加 | ListOpenSlots, ReserveSlot（UPDATE … WHERE booked_count < capacity RETURNING）, ReleaseSlot | 2.1, 2.2 |
| backend/db/query/appointment.sql | 追加 | Create/ListByPatient/Cancel/Reschedule 用 | 2.2 |
| backend/db/query/queue.sql | 追加 | Enqueue, Transition, CountAhead, RecentDurations（直近20件）, ListForStaff | 2.3, 3.1 |
| backend/db/query/notification.sql | 追加 | Enqueue, ClaimPending（FOR UPDATE SKIP LOCKED）, MarkSent/Failed | 2.4 |
| backend/db/query/admin.sql | 追加 | schedules/settings/closures/audit | 2.1, 3.2, #6 |
| backend/internal/repository/*_repository.go | 追加 | SlotRepository, AppointmentRepository, QueueRepository, NotificationRepository, AuditRepository の interface（ctx 第一引数） | 全 service |
| backend/internal/repository/mocks/* | 追加（生成物） | mockery | テスト |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### SlotRepository.Reserve

**入力**
```
入力: ctx, slotID uuid
  例: 7b1c…
```

**出力**
```
出力: (reserved bool, err error)。0 行更新なら reserved=false
```

**エラー**
DB 障害 → wrapped error

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| sqlc | 最新安定 | 生成 |
| pgx/v5 | v5 | ドライバ |
| mockery | v2 | モック生成 |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| 症状メモを含む列を公開用クエリで SELECT | 要配慮情報 | 公開用クエリは専用の列のみ |

---

## 6. タスク詳細

### TASK-0001: sqlc クエリと repository interface（clinic-reservation）

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-002-AC-2: WHEN 同一枠に複数の予約が同時に送信された THEN システムは排他制御により定員を超える予約を確定せず、後続に HTTP 409 を返す（Must）
- [WHEN/IF] REQ-004-AC-2: システムは待ち時間を直近の診察実績（平均診察時間）と待ち人数から算出する（Must）

**実装手順**
1. 設計書の API/サービス単位でクエリを分割して書く（読取/書込の分離）
2. ReserveSlot を単一 UPDATE … RETURNING で実装する
3. 待機列の CountAhead と RecentDurations を設計書の式に合わせる
4. repository interface を定義（Queries は直接注入しない）
5. mockery 設定と生成、CI で generate 差分チェック

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 空き枠で ReserveSlot が 1 行更新 |
| 異常系 | 満席枠で 0 行 → reserved=false |
| 境界値 | capacity=1 に対する同時 2 リクエストで成功は 1 件のみ |

**受け入れ基準チェックリスト**
- [ ] 正常系: ReserveSlot が空き枠で成功
- [ ] 異常系: 満席枠で false
- [ ] 境界値: capacity=1 に 20 並行で成功 1 件
- [ ] 正常系: ClaimPending が複数ワーカーで重複取得されない
- [ ] セキュリティ: 公開用クエリが symptom_note を返さない

**依存関係**
- 前提: #2（sqlc 設定）, #4（スキーマ）
- 後続: user-auth を除く P1 全タスク

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
| Unit | モックでの契約確認 | testify + mockery |
| Integration | 並行 ReserveSlot（goroutine 20 本）で定員超過なし | go test + 一時 PostgreSQL |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: ReserveSlot が空き枠で成功
- [ ] 異常系: 満席枠で false
- [ ] 境界値: capacity=1 に 20 並行で成功 1 件
- [ ] 正常系: ClaimPending が複数ワーカーで重複取得されない
- [ ] セキュリティ: 公開用クエリが symptom_note を返さない

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
interface 契約が実装と合わずに P1 が大きく手戻りする場合。

### 10.2 ロールバック手順
interface とクエリを前コミットへ。P1 未着手の間は影響なし。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| P1 着手後に interface を変更し複数タスクが手戻り | H | M | P1 開始前に interface をレビュー・凍結する |
| #2 と sqlc.yaml を同時編集して競合 | M | M | #2 を先にマージ、本 Issue は追記のみ |
| 待機列の sort_time 規則をクエリ側と service 側で二重定義 | M | M | ORDER BY sort_time のみに統一し、規則は Enqueue 時に確定 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] interface が service ごとに最小になっていて、巨大な Repository に集約されていないか

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
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T16:13:39+09:00 |
