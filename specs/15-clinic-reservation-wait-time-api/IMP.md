---
kairo:
  node_id: "imp:15-clinic-reservation-wait-time-api"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "15-clinic-reservation-wait-time-api"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T18:28:57+09:00"
  updated_at: "2026-10-10T18:28:57+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:15-clinic-reservation-wait-time-api"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/wait_time_service.go', 'backend/internal/handler/waittime_handler.go', 'backend/db/query/queue.sql', 'backend/internal/app/app.go']
  band: "Green"
  last_validated: "2026-10-10T18:28:57+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 15
---

# IMP: 待ち時間算出と公開 API

| 項目 | 値 |
|---|---|
| imp_id | IMP-15-clinic-reservation-wait-time-api |
| imp_version | 1.0.0 |
| source_issue | #15（https://github.com/kava2108/booking/issues/15） |
| task | clinic-reservation tasks.md §3.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> 診療科ごとの現在の待ち人数と推定待ち時間を算出し、未ログインでも見られる公開 API と、予約患者本人の順番・呼出見込み API を実装する。
> 待ち時間 = ceil(待ち人数 × 平均所要時間 ÷ 診察中の医師数)。実績が 3 件未満なら標準診察時間（basis=standard）、無ければ「算出不可」と理由を返し、数値を推測しない。
> 公開応答は 10 秒キャッシュし、氏名・受付番号などの個人情報を含めない。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 このシステムの核である待ち時間の表示が無い。🔵 実績が少ない開院直後や休診中に、誤った数値を出してはならない（REQ-004-AC-5）。

### 1.2 目標
診療科別の待ち時間が未ログインで取得でき、算出不可の理由が区別され、キャッシュしても 30 秒以内の更新要件を満たす。

### 1.3 非目標（Out of Scope）
画面（P3 4.1）、休診の設定（#16）、医師別の待ち時間。

---

## 2. スニペット（変更対象の既存コード）

#5 の QueueRepository（CountWaiting, CountAhead, RecentConsultSeconds, CountOnDutyDoctors, GetQueueEntryForPatient）。#13 の待機列。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/wait_time_service.go | 追加 | 診療科別の算出、順番と呼出見込み、TTL キャッシュ | P3 4.1 |
| backend/internal/handler/waittime_handler.go | 追加 | GET /wait-times（公開）、GET /appointments/{id}/position（本人） | — |
| backend/db/query/queue.sql | 修正 | CountScheduledDoctors, CountDoctorsStartingLater を追加（休診と診療時間外の区別） | — |
| backend/internal/app/app.go | 修正 | 結線（公開レートリミット） | — |

### 3.2 API 変更

GET /api/v1/wait-times、GET /api/v1/appointments/{id}/position（実装。定義は #6 で作成済み）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### GET /api/v1/wait-times

**入力**
```
入力: なし（認証不要）
```

**出力**
```
出力: {updated_at, note, departments:[{department_id,name,status,waiting_count,wait_minutes,basis,reason}]}
```

**エラー**
レートリミット 429

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
| 公開応答に氏名・受付番号・症状メモ | REQ-007-AC-1 | 件数と分のみ |
| データ不足時に推測した数値を返す | REQ-004-AC-5 | unavailable + reason |

---

## 6. タスク詳細

### TASK-0001: 待ち時間算出と公開 API

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-004-AC-1: WHEN 誰かが待ち時間ページを開く THEN システムは診療科ごとの現在の待ち人数と推定待ち時間（分）を、未ログインでも表示する（Must）
- [WHEN/IF] REQ-004-AC-2: システムは待ち時間を直近の診察実績（平均診察時間）と待ち人数から算出する（Must）
- [WHEN/IF] REQ-004-AC-4: WHEN 予約済み患者が自身の予約詳細を開く THEN システムは自分の順番（あと N 人）と呼出見込み時刻を表示する（Should）
- [WHEN/IF] REQ-004-AC-5: IF 待ち時間を算出できない（診療開始前・休診・診察実績が3件未満かつ標準診察時間も未設定）THEN システムは「算出不可」と理由を表示し、数値を推測表示しない。診察実績が3件未満でも管理者設定の標準診察時間がある場合は、それを用いて算出し「標準時間による目安」と明示する（Must）
- [WHEN/IF] REQ-004-AC-6: 待ち時間表示は目安である旨の注記を併記する（Must）
- [WHEN/IF] REQ-007-AC-1: 待ち時間の公開表示には個人を特定できる情報（氏名等）を含めない（Must）

**実装手順**
1. 診療科ごとに 診療状況（診察中の医師数・休診・開始前）を判定
2. 平均所要時間（直近 20 件、3 件未満は標準診察時間）
3. 待ち時間 = ceil(待ち人数 × 平均 ÷ 医師数)
4. 順番と呼出見込み（本人のみ）
5. 10 秒 TTL キャッシュ（並行アクセスでも算出は 1 回）

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 待ち 6 人・平均 10 分・医師 2 人 → 30 分 |
| 異常系 | 休診中は unavailable(closed)、診療開始前は before_hours、データ無しは no_data |
| 境界値 | 実績 2 件 / 3 件の切替、待ち 0 人は 0 分、TTL ちょうど |

**受け入れ基準チェックリスト**
- [ ] 正常系: 式どおりの分数（切り上げ）
- [ ] 境界: 実績 3 件で actual、2 件で standard
- [ ] 異常系: 標準時間も無ければ no_data、休診 closed、開始前 before_hours
- [ ] 正常系: 待ち 0 人は 0 分
- [ ] セキュリティ: 公開応答に個人情報が無い／position は本人のみ（他人は 404）
- [ ] 正常系: 10 秒キャッシュ（期間内は再計算しない、超過で更新）
- [ ] OpenAPI 契約に合致

**依存関係**
- 前提: #13
- 後続: P3 4.1

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
| Integration | 実 DB・契約チェック | pgtest + httptest |
| E2E | P4 | Playwright |

### 8.2 テスト観点

- [ ] 正常系: 式どおりの分数（切り上げ）
- [ ] 境界: 実績 3 件で actual、2 件で standard
- [ ] 異常系: 標準時間も無ければ no_data、休診 closed、開始前 before_hours
- [ ] 正常系: 待ち 0 人は 0 分
- [ ] セキュリティ: 公開応答に個人情報が無い／position は本人のみ（他人は 404）
- [ ] 正常系: 10 秒キャッシュ（期間内は再計算しない、超過で更新）
- [ ] OpenAPI 契約に合致

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
誤った待ち時間が表示される場合。

### 10.2 ロールバック手順
公開ルートを無効化（患者には「お問い合わせください」の表示）。DB 変更なし。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| スタッフの記録漏れで待ち時間が不正確 | H | M | 「目安」の注記を必須表示。算出不可を正しく返す |
| キャッシュ期間中に状態が変わる | L | H | 10 秒。要件の 30 秒更新内 |
| 医師数が 1 を下回る計算 | M | L | 診察中 0 のときは unavailable。0 除算を防ぐ |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] 算出ロジックが 1 箇所（公開 API と順番表示で共通）か

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
