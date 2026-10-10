---
kairo:
  node_id: "imp:16-clinic-reservation-closures"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "16-clinic-reservation-closures"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T18:28:57+09:00"
  updated_at: "2026-10-10T18:28:57+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:16-clinic-reservation-closures"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/closure_service.go', 'backend/internal/handler/closure_handler.go', 'backend/internal/domain/notification.go', 'backend/internal/service/notification_templates.go']
  band: "Green"
  last_validated: "2026-10-10T18:28:57+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 16
---

# IMP: 休診・医師不在の設定

| 項目 | 値 |
|---|---|
| imp_id | IMP-16-clinic-reservation-closures |
| imp_version | 1.0.0 |
| source_issue | #16（https://github.com/kava2108/booking/issues/16） |
| task | clinic-reservation tasks.md §3.2 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> 受付スタッフが休診・医師不在を設定すると、該当期間の枠を closed にし、既存の予約者へ通知を登録する。
> 休診中は待ち時間が「算出不可（closed）」になる（#15）。操作は監査ログに記録する。
> 通知はアウトボックス経由（送信は #14 のワーカー）。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 臨時休診・医師不在の運用手段が無い。🔵 既存の予約者に知らせずに休診すると、患者が無駄足になる。

### 1.2 目標
休診設定で枠が予約不可になり、影響する予約者全員に通知が登録され、同一 Tx で完結する。

### 1.3 非目標（Out of Scope）
休診の取消・編集、予約の自動キャンセル・自動振替、画面。

---

## 2. スニペット（変更対象の既存コード）

#5 の CreateClosure / CloseSlots / ListBookedAppointmentsForNotice、#14 の通知ワーカー。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/closure_service.go | 追加 | 入力検証、休診作成・枠の closed 化・通知登録・監査（同一 Tx） | P3 4.2 |
| backend/internal/handler/closure_handler.go | 追加 | POST /staff/closures（StaffOnly） | — |
| backend/internal/domain/notification.go | 修正 | clinic_closure 種別 | — |
| backend/internal/service/notification_templates.go | 修正 | 休診のお知らせ本文（理由の自由記述は含めない） | — |

### 3.2 API 変更

POST /api/v1/staff/closures（実装）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### POST /api/v1/staff/closures

**入力**
```
入力: {doctor_id?, from_at, to_at, reason}
```

**出力**
```
出力: 201 {closed_slots, notified}
```

**エラー**
期間不正・過去・未知の医師 422

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
| 休診の理由（自由記述）を通知本文に含める | 意図しない情報の送信 | 定型文のみ |
| 予約の自動キャンセル | 要件外。患者の意思確認が必要 | 通知のみ |

---

## 6. タスク詳細

### TASK-0001: 休診・医師不在の設定

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-005-AC-3: WHEN スタッフが休診・臨時休診・医師不在を設定する THEN システムは該当枠を予約不可にし、既存予約者へ通知する（Must）

**実装手順**
1. 入力検証（from<to、終了が現在より後、医師の存在、最大期間）
2. Tx: 休診作成 → 枠を closed → 影響する予約者へ通知 → 監査ログ
3. 結果（閉じた枠数・通知数）を返す

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 医師 A の午前を休診 → 該当枠が closed、予約者 N 人に通知 |
| 異常系 | from>=to・終了が過去・未知の医師は 422、患者は 403 |
| 境界値 | 期間の終了ちょうどの枠は対象外（[from,to)） |

**受け入れ基準チェックリスト**
- [ ] 正常系: 枠が closed・通知が予約者数ぶん登録
- [ ] 正常系: 全体休診（doctor_id なし）は全医師が対象
- [ ] 異常系: 検証エラー時は何も変更しない
- [ ] 境界: [from,to) の端
- [ ] セキュリティ: StaffOnly、操作を監査ログに記録
- [ ] 正常系: 休診後の待ち時間は unavailable(closed)

**依存関係**
- 前提: #11, #12, #14
- 後続: P3 4.2

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
| Integration | 実 DB（原子性・監査・待ち時間への反映） | pgtest + httptest |
| E2E | P4 | Playwright |

### 8.2 テスト観点

- [ ] 正常系: 枠が closed・通知が予約者数ぶん登録
- [ ] 正常系: 全体休診（doctor_id なし）は全医師が対象
- [ ] 異常系: 検証エラー時は何も変更しない
- [ ] 境界: [from,to) の端
- [ ] セキュリティ: StaffOnly、操作を監査ログに記録
- [ ] 正常系: 休診後の待ち時間は unavailable(closed)

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
誤って休診を設定した場合。

### 10.2 ロールバック手順
closures の行を削除し、該当枠を open に戻す SQL を実行（通知は撤回できない→お詫び連絡）。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| 誤設定で多数の予約者に通知が飛ぶ | H | M | 確認画面・期間上限。取消機能は TODO |
| 通知後も予約が booked のまま残る | M | H | 要件どおり。スタッフが個別対応。キャンセル期限の例外は TODO |
| 同じ期間の重複設定で二重通知 | M | M | 重複期間の検出は TODO（現状は別の休診として扱う） |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] 休診の反映（枠・通知・監査）が単一 Tx で完結しているか

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
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T18:28:57+09:00 |
