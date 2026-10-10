---
kairo:
  node_id: "imp:11-clinic-reservation-slot-api-admin-settings"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "11-clinic-reservation-slot-api-admin-settings"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T17:38:15+09:00"
  updated_at: "2026-10-10T17:38:15+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:11-clinic-reservation-slot-api-admin-settings"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/slot_service.go', 'backend/internal/service/admin_service.go', 'backend/internal/handler/catalog_handler.go', 'backend/internal/handler/admin_handler.go', 'backend/db/query/admin.sql,', 'backend/internal/domain/clock.go', 'backend/api/openapi.yaml,', 'backend/internal/app/app.go,']
  band: "Green"
  last_validated: "2026-10-10T17:38:15+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 11
---

# IMP: 診療科・医師・枠 API と管理者設定

| 項目 | 値 |
|---|---|
| imp_id | IMP-11-clinic-reservation-slot-api-admin-settings |
| imp_version | 1.0.0 |
| source_issue | #11（https://github.com/kava2108/booking/issues/11） |
| task | clinic-reservation tasks.md §2.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> 診療科・医師・空き枠の公開 API と、診療スケジュール・各種設定の管理者 API を実装する。
> SlotService は doctor_schedules から枠を冪等に生成し（日次ジョブ + 管理者操作時）、休診中の期間は closed にする。
> 満席・休診・締切後の枠も unavailable_reason 付きで返し、選択不可として表示できるようにする。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 予約の前提となる枠が存在しない。🔵 休診設定後に生成された枠が open のままだと予約できてしまう。

### 1.2 目標
公開 API が認証なしで動き（レートリミット付き）、管理者が設定したスケジュールから枠が生成され、設定が予約ルール（受付期間・キャンセル期限）として保存される。

### 1.3 非目標（Out of Scope）
予約確定（#12）、休診の設定 API（P2 3.2）、画面（P2/P3）。

---

## 2. スニペット（変更対象の既存コード）

#5 の SlotRepository / AdminRepository（ListSlots, CreateSlotIfAbsent, CloseSlots, ListSchedulesByWeekday, UpdateClinicSettings ほか）。#6 の AdminOnly。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/slot_service.go | 追加 | ListSlots（可否判定）、GenerateSlots（冪等・休診反映）、SlotGenerator.Run（定期実行） | #12 |
| backend/internal/service/admin_service.go | 追加 | PutSchedules（検証・置換・枠生成）、PutSettings | — |
| backend/internal/handler/catalog_handler.go | 追加 | GET departments / doctors / slots | — |
| backend/internal/handler/admin_handler.go | 追加 | PUT schedules / settings（AdminOnly） | — |
| backend/db/query/admin.sql, slot.sql | 修正 | ListClosuresFrom, GetDepartment, GetDoctor を追加 | — |
| backend/internal/domain/clock.go | 追加 | JST（FixedZone）と日付境界ヘルパー | #12, #13, #14 |
| backend/api/openapi.yaml, frontend/src/api/schema.d.ts | 修正 | unavailable_reason に outside_window を追加 | — |
| backend/internal/app/app.go, cmd/api/main.go | 修正 | ルート結線と枠生成ジョブの起動 | — |

### 3.2 API 変更

GET /api/v1/{departments,doctors,slots}、PUT /api/v1/admin/{schedules,settings}（実装。定義は #6 で作成済み）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### GET /api/v1/slots

**入力**
```
入力: department_id=1, doctor_id（任意）, date=2026-10-12
```

**出力**
```
出力: [{id, doctor_id, start_at, available, unavailable_reason?}] 時刻順
```

**エラー**
date 不正/department_id 無し 422

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
| time.Local / LoadLocation への依存 | tzdata 無しの環境で失敗 | FixedZone(JST) |
| 公開 API の応答に capacity / booked_count を含める | 内部情報 | available と reason のみ |

---

## 6. タスク詳細

### TASK-0001: 診療科・医師・枠 API と管理者設定

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-001-AC-1: WHEN 患者が診療科と日付を指定する THEN システムは予約可能な枠を時刻順に表示する（Must）
- [WHEN/IF] REQ-001-AC-2: WHEN 医師を指名する THEN システムはその医師の枠のみを表示する（Must）
- [WHEN/IF] REQ-001-AC-3: WHEN 枠が満席・休診・受付締切後である THEN システムはその枠を選択不可として表示する（Must）
- [WHEN/IF] REQ-006-AC-1: WHEN 管理者が診療時間・枠の長さ・定員を設定する THEN システムは該当日の予約枠を生成する（Must）
- [WHEN/IF] REQ-006-AC-2: WHEN 管理者が予約受付期間・キャンセル期限を設定する THEN システムは以降の予約に適用する（Should）

**実装手順**
1. domain/clock.go（JST と日付境界）を作る
2. SlotService.ListSlots: 枠取得 → full/closed/past_cutoff/outside_window を判定
3. GenerateSlots: スケジュール × 日付で CreateSlotIfAbsent、生成後に休診期間へ CloseSlots
4. AdminService: スケジュール検証（曜日/時刻/重複）→ 置換 → 枠生成、設定の検証と保存
5. ハンドラーと結線、日次ジョブ

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 月曜 9:00-12:00 30 分枠なら 6 枠生成される |
| 異常系 | end<=start・重複スケジュール・不正な曜日は 422、存在しない医師は 404 |
| 境界値 | 終了時刻ちょうど開始の枠は作らない／受付期間の最終日の枠は可・翌日は outside_window |

**受け入れ基準チェックリスト**
- [ ] 正常系: 空き枠検索が時刻順で、満席/休診/過去が unavailable_reason 付き
- [ ] 正常系: 医師指名で絞り込める
- [ ] 正常系: 生成が冪等（2 回で増えない）
- [ ] 正常系: 休診期間の枠は closed で生成される
- [ ] 異常系: 不正入力 422、権限なしで 403（patient/staff）
- [ ] 境界値: 終了ちょうど・受付期間の境界
- [ ] セキュリティ: 公開 API に内部カウントを出さない／管理者 API は clinic_admin のみ

**依存関係**
- 前提: #4, #5, #6
- 後続: #12

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
| Integration | 生成の冪等性・休診反映・権限 | pgtest + httptest |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: 空き枠検索が時刻順で、満席/休診/過去が unavailable_reason 付き
- [ ] 正常系: 医師指名で絞り込める
- [ ] 正常系: 生成が冪等（2 回で増えない）
- [ ] 正常系: 休診期間の枠は closed で生成される
- [ ] 異常系: 不正入力 422、権限なしで 403（patient/staff）
- [ ] 境界値: 終了ちょうど・受付期間の境界
- [ ] セキュリティ: 公開 API に内部カウントを出さない／管理者 API は clinic_admin のみ

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
不正な枠が大量生成された場合。

### 10.2 ロールバック手順
未予約の枠のみ削除する SQL を手動実行（予約済みは触らない）。ジョブを停止。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| 生成ジョブが重複実行されても冪等だが負荷が増える | L | M | ON CONFLICT DO NOTHING。実行間隔は 1 時間 |
| スケジュール変更で既存の予約済み枠との不整合 | M | M | 予約済みの枠は削除しない。新規生成のみ（旧スケジュールの未予約枠は残る→TODO） |
| JST 固定が他地域で破綻 | L | L | 単一医療機関・国内前提と明記 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] SlotService が repository interface のみに依存し、時刻は注入可能か

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
