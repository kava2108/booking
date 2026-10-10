---
kairo:
  node_id: "imp:14-clinic-reservation-notification-outbox"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "14-clinic-reservation-notification-outbox"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T17:38:15+09:00"
  updated_at: "2026-10-10T17:38:15+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:14-clinic-reservation-notification-outbox"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/notification_worker.go', 'backend/internal/service/notification_templates.go', 'backend/internal/service/reminder_scheduler.go', 'backend/internal/service/mailer.go', 'backend/db/query/notification.sql,', 'backend/cmd/api/main.go']
  band: "Green"
  last_validated: "2026-10-10T17:38:15+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 14
---

# IMP: 通知基盤（アウトボックス・メール・リマインド）

| 項目 | 値 |
|---|---|
| imp_id | IMP-14-clinic-reservation-notification-outbox |
| imp_version | 1.0.0 |
| source_issue | #14（https://github.com/kava2108/booking/issues/14） |
| task | clinic-reservation tasks.md §2.4 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> アウトボックスの通知をメールで送信するワーカー、前日リマインド、順番接近通知を実装する。
> リース方式で重複なく取得し、失敗は指数バックオフで最大 5 回再試行。本文に症状メモ等の要配慮情報を含めない。
> 送信は Sender interface 越し（初期は SMTP。SMS/LINE は将来差し替え）。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 #12/#13 が登録した通知を誰も送らない。🔵 外部 SMTP の遅延・障害が予約処理に影響してはならない。

### 1.2 目標
登録された通知が送信され、失敗は再試行→failed になる。リマインドと「あと N 人」は 1 件につき 1 回だけ。

### 1.3 非目標（Out of Scope）
SMS/LINE 実装、休診通知の登録（P2 3.2）、通知設定画面。

---

## 2. スニペット（変更対象の既存コード）

#5 の NotificationRepository（ClaimNotifications はリース）。#8 の SMTPMailer。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/notification_worker.go | 追加 | Claim → 描画 → 送信 → Sent/Failed/再スケジュール。Run ループ | — |
| backend/internal/service/notification_templates.go | 追加 | 種別ごとの件名・本文（JST 表記、症状メモを含めない） | — |
| backend/internal/service/reminder_scheduler.go | 追加 | 前日 18:00(JST) 以降のリマインド登録、順番接近通知の登録（冪等） | — |
| backend/internal/service/mailer.go | 修正 | SMTPMailer に汎用 Send を追加 | — |
| backend/db/query/notification.sql, appointment.sql, queue.sql | 修正 | RescheduleNotification, ListAppointmentsNeedingReminder, ListQueueEntriesNeedingNearNotice を追加 | — |
| backend/cmd/api/main.go | 修正 | ワーカー・スケジューラの起動と停止 | — |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### NotificationWorker.RunOnce

**入力**
```
入力: ctx（Claim 上限 20、リース 5 分）
```

**出力**
```
出力: 処理件数。送信成功 → sent、失敗 → attempts<5 なら send_after を後ろ倒し、5 回目で failed
```

**エラー**
宛先ユーザー不明・未知の種別 → failed（再試行しない）

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
| 通知本文・ログに症状メモ・診断名を含める | REQ-007-AC-4 | 予約番号・日時・診療科・医師名のみ |
| 宛先メールアドレスのログ出力 | 個人情報 | 通知 ID と種別のみ |
| Tx を保持したままの送信 | ロック長期化 | リース方式 |

---

## 6. タスク詳細

### TASK-0001: 通知基盤（アウトボックス・メール・リマインド）

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-002-AC-3: WHEN 予約が確定した THEN システムは確認通知（日時・診療科・予約番号）をメールで送信する（Must）
- [WHEN/IF] REQ-002-AC-6: WHEN 予約の前日と、自分の順番が近づいた時点（あと数人）に達した THEN システムはリマインド通知を送信する（Should）
- [WHEN/IF] REQ-003-AC-2: WHEN 患者がキャンセル期限内に予約をキャンセルする THEN システムは枠を解放し通知を送信する（Must）
- [WHEN/IF] REQ-007-AC-4: 個人情報保護法および医療情報システムの安全管理ガイドラインに準拠する（Must）

**実装手順**
1. Sender interface と SMTPMailer.Send
2. テンプレート（種別ごと、JST 表記、症状メモ無し）
3. Worker: Claim → 宛先取得 → 描画 → 送信 → 結果更新（バックオフ）
4. ReminderScheduler: 前日 18:00 以降に未登録の予約を登録、順番接近を登録（payload で冪等）
5. main で起動・graceful stop

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 登録 → 送信 → sent、リマインドは 1 予約 1 回 |
| 異常系 | SMTP 失敗で再試行、5 回で failed／宛先不明は即 failed |
| 境界値 | attempts=5 の失敗で failed／前日 17:59 は登録せず 18:00 で登録 |

**受け入れ基準チェックリスト**
- [ ] 正常系: 各種別の本文に日時・診療科・予約番号が入る
- [ ] セキュリティ: どの本文・ログにも症状メモが無い
- [ ] 異常系: 送信失敗のバックオフと failed
- [ ] 並行: 複数ワーカーで重複送信しない
- [ ] 正常系: リマインド/順番接近が二重登録されない
- [ ] 境界値: 18:00 JST の境界、あと N 人の N 境界
- [ ] 異常系: 宛先不明は failed で再試行しない

**依存関係**
- 前提: #12, #13, #5, #8
- 後続: P2 3.2

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
| Unit | 90% | testify + fake Sender |
| Integration | Claim 重複なし・冪等な登録 | pgtest |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: 各種別の本文に日時・診療科・予約番号が入る
- [ ] セキュリティ: どの本文・ログにも症状メモが無い
- [ ] 異常系: 送信失敗のバックオフと failed
- [ ] 並行: 複数ワーカーで重複送信しない
- [ ] 正常系: リマインド/順番接近が二重登録されない
- [ ] 境界値: 18:00 JST の境界、あと N 人の N 境界
- [ ] 異常系: 宛先不明は failed で再試行しない

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
誤送信・大量送信が起きた場合。

### 10.2 ロールバック手順
ワーカーを停止し、pending を failed に更新する SQL を実行。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| 同一通知の二重送信（送信後・更新前のクラッシュ） | M | L | at-least-once として許容。冪等キーは将来 |
| メール到達率（SPF/DKIM） | M | M | 運用設定。本 Issue の範囲外 |
| リマインドの JST 前提 | L | L | 単一医療機関・国内前提 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] Sender の差し替え（SMS/LINE）で worker が変更不要か

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
