---
kairo:
  node_id: "imp:17-clinic-reservation-frontend-booking"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "17-clinic-reservation-frontend-booking"
  feature: "clinic-reservation"
  imp_version: "1.0.1"
  status: "active"
  created_at: "2026-10-10T20:10:00+09:00"
  updated_at: "2026-10-10T20:40:00+09:00"
  drift_baseline: "e67336bf4d6f20eada9cbf37744f4b5d2eb4027b"
coherence:
  id: "imp:17-clinic-reservation-frontend-booking"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['frontend/src/features/appointment', 'frontend/src/features/catalog', 'frontend/src/pages/ReservePage.tsx', 'frontend/src/pages/MyPage.tsx']
  band: "Green"
  last_validated: "2026-10-10T20:10:00+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 17
---

# IMP: フロント 検索・予約・マイページ

| 項目 | 値 |
|---|---|
| imp_id | IMP-17-clinic-reservation-frontend-booking |
| imp_version | 1.0.1 |
| source_issue | #17（https://github.com/kava2108/booking/issues/17） |
| task | clinic-reservation tasks.md §3.3 |
| status | active（**実装済みのコードから後追いで作成**） |
| drift_baseline | e67336bf4d6f20eada9cbf37744f4b5d2eb4027b |
| author | Suetsugu Kawamura |

> **Executive Summary**
> 患者が診療科・医師・日付で空き枠を探して予約し、マイページで予約の確認・日時変更・キャンセルをするフロントエンド。
> 満席・休診・締切後の枠は理由つきで選択不可にし、満席で失敗したときは枠を最新にして選び直せる。
> 本 IMP は実装（PR #18）の後に作成したため、記述は実装と vitest / Playwright の内容に基づく。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 患者が画面から予約・変更・キャンセルできない。🔵 満席や休診の枠を押してしまうと無駄な失敗になる。

### 1.2 目標
REQ-001〜003 の受け入れ基準（Must 4 / Should 4 件のうち、AC は issue-struct.md の 8 件）を画面で満たす。

### 1.3 非目標（Out of Scope）
待ち時間ボード・当日受付（#18 系の 4.1）、スタッフ画面（4.2）、管理者画面（4.3）。バックエンド API の変更（#11, #12 が所有）。

---

## 2. スニペット（変更対象の既存コード）

利用する API は backend/api/openapi.yaml 由来の frontend/src/api/schema.d.ts（`/slots`, `/appointments`, `/appointments/me`, `/appointments/{id}` の DELETE / PATCH）。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| frontend/src/features/catalog/api/useCatalog.ts | 追加 | useDepartments / useDoctors / useSlots（公開 API） | 予約・変更 |
| frontend/src/features/appointment/api/useAppointments.ts | 追加 | 一覧・作成・キャンセル・変更の React Query フック。変更後は slots / appointments / position を無効化 | マイページ |
| frontend/src/features/appointment/components/SlotPicker.tsx | 追加 | 医師別の枠一覧。不可枠は理由つきで disabled | 予約・変更 |
| frontend/src/features/appointment/components/BookingFlow.tsx | 追加 | 診療科・医師・日付の検索 | /reserve |
| frontend/src/features/appointment/components/BookingDialog.tsx | 追加 | 予約フォーム（react-hook-form + zod）。未ログインはログイン案内 | /reserve |
| frontend/src/features/appointment/components/AppointmentList.tsx, AppointmentCard.tsx | 追加 | 今後 / 過去の一覧、期限超過の案内 | /mypage |
| frontend/src/features/appointment/components/CancelDialog.tsx, RescheduleDialog.tsx | 追加 | キャンセル確認、日時変更 | /mypage |
| frontend/src/pages/ReservePage.tsx, MyPage.tsx | 追加 | ページ枠 | ルーティング |

### 3.2 API 変更

| エンドポイント | 変更前 | 変更後 | 破壊的変更? |
|---|---|---|---|
| GET /api/v1/departments, /doctors, /slots | 既存 | 利用のみ | なし |
| POST /api/v1/appointments | 既存 | 利用のみ | なし |
| GET /api/v1/appointments/me | 既存 | 利用のみ | なし |
| PATCH / DELETE /api/v1/appointments/{id} | 既存 | 利用のみ | なし |
| GET /api/v1/appointments/{id}/position | 既存（#15） | 予約カード内の PositionPanel（features/waittime）が本日の予約で利用。本 IMP では実装しない | なし |

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### 予約の作成（BookingDialog → POST /appointments）

**入力**
```
slot_id: string, visit_type: "first" | "revisit"（必須）, symptom_note: string（任意・1000 文字以内）
```

**出力**
```
201 Appointment { id, appointment_no, slot_start, ... } → 予約番号を画面に表示
```

**エラー**
| エラー種別 | 条件 | 画面の挙動 |
|---|---|---|
| 409 | 満席 / 同じ時間帯に予約済み | メッセージを表示し、slots を再取得して選び直せる |
| 401 | 未ログイン | 予約フォームを出さずログインへ誘導（戻り先 /reserve） |

### 日時の変更（RescheduleDialog → PATCH /appointments/{id}）

**入力**: `{ slot_id }`（同じ診療科の別の枠）。**出力**: 200 Appointment。
**エラー**: 409（変更先が満席）→ メッセージ表示、元の予約は変わらない。新旧枠の原子的な入れ替えはサーバー側（#12）が保証する。

### キャンセル（CancelDialog → DELETE /appointments/{id}）

期限内（`cancelable: true`）のみ操作を表示。期限超過（422）はメッセージを表示する。

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| @tanstack/react-query | ^5 | サーバー状態 |
| react-hook-form + @hookform/resolvers + zod | ^7 / ^5 / ^4 | 予約フォーム |
| axios（src/api/client.ts） | ^1 | HTTP |
| lucide-react | ^1 | アイコン |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| コンポーネントからの直接 HTTP 呼び出し | api/ への集約（tasks.md 3.3） | features/*/api のフック |
| 症状メモのログ・URL・localStorage への保存 | 要配慮情報 | 送信時のみメモリ上に保持 |
| dangerouslySetInnerHTML | XSS | React のエスケープ |
| 色の直書き | デザインシステム準拠 | Tailwind のトークン（design-system/MASTER.md） |

---

## 6. タスク詳細

### TASK-0001: 空き枠の検索と表示（REQ-001）

**完了条件（EARS 記法）**
- [WHEN] REQ-001-AC-1: 患者が診療科と日付を指定する [THE SYSTEM SHALL] 予約可能な枠を時刻順に表示する
- [WHEN] REQ-001-AC-2: 医師を指名する [THE SYSTEM SHALL] その医師の枠のみを表示する
- [WHEN] REQ-001-AC-3: 枠が満席・休診・受付締切後である [THE SYSTEM SHALL] その枠を選択不可として表示する

**実装手順**
1. useDepartments / useDoctors / useSlots を features/catalog/api に実装
2. SlotPicker: 医師ごとにまとめ、不可枠は理由（満席・休診・受付終了・予約受付期間外）つきで disabled
3. BookingFlow: 診療科→医師（任意）→日付（今日より前は不可）

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 診療科を選ぶと当日の枠を時刻順に表示 |
| 異常系 | 取得失敗でエラーと再試行 |
| 境界値 | 枠が無い日 / すべて不可の日は別の日を案内。時刻は JST 固定 |

**受け入れ基準チェックリスト**
- [x] REQ-001-AC-1 / AC-2 / AC-3

**依存関係**: 前提 #11 / 後続 TASK-0002

### TASK-0002: 予約の確定（REQ-002）

**完了条件（EARS 記法）**
- [WHEN] REQ-002-AC-1: ログイン済み患者が空き枠・来院区分・症状メモを送信する [THE SYSTEM SHALL] 予約を確定し予約番号を発行する

**実装手順**
1. BookingDialog（react-hook-form + zod、来院区分必須、症状メモ 1000 文字以内）
2. 二重送信防止（送信中は loading）、成功時に予約番号を表示、409 で枠を再取得

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 確定して予約番号を表示 |
| 異常系 | 満席 409 / 同時間帯の重複 409 / 未ログインはログイン案内 |
| 境界値 | 症状メモ 1000 文字ちょうど可、1001 文字不可 |

**受け入れ基準チェックリスト**
- [x] REQ-002-AC-1

**依存関係**: 前提 #12, TASK-0001

### TASK-0003: マイページ（REQ-003）

**完了条件（EARS 記法）**
- [WHEN] REQ-003-AC-1: 患者がマイページを開く [THE SYSTEM SHALL] 自分の予約一覧（今後・過去）を表示する
- [WHEN] REQ-003-AC-2: 患者がキャンセル期限内に予約をキャンセルする [THE SYSTEM SHALL] 枠を解放し通知を送信する（画面は確認→実行→一覧更新。解放・通知はサーバー側）
- [WHEN] REQ-003-AC-3: キャンセル期限を過ぎている [THE SYSTEM SHALL] キャンセル不可とし、医療機関への連絡方法を案内する
- [WHEN] REQ-003-AC-4: 患者が日時を変更する [THE SYSTEM SHALL] 新枠の確保と旧枠の解放を原子的に行う（画面は変更ダイアログ。原子性はサーバー側）

**実装手順**
1. AppointmentList / AppointmentCard（今後と過去・取消済みを分離、cancelable で操作を出し分け）
2. CancelDialog（確認画面付き）、RescheduleDialog（同一診療科の別枠）
3. 変更後は slots / appointments / position を無効化

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 一覧表示、キャンセル、日時変更 |
| 異常系 | 期限超過 422 のメッセージ、変更先満席 409 で元の予約は不変、一覧取得失敗は再試行 |
| 境界値 | 予約ゼロの案内、本日の予約のみ順番パネル |

**受け入れ基準チェックリスト**
- [x] REQ-003-AC-1 / AC-2 / AC-3 / AC-4

**依存関係**: 前提 #12, TASK-0002

---

## 7. 禁止事項

- [ ] 本 Issue の「3.1 変更ファイル一覧」以外のファイルを変更しないこと
- [ ] 他 Issue が所有するバックエンドのファイル・マイグレーションを変更しないこと
- [ ] 既存のテストを削除・修正しないこと
- [ ] 新たな外部依存を追加しないこと（5.1 に無いもの。必要なら IMP を更新）
- [ ] 禁止パターン（5.2）を使わないこと

---

## 8. テスト戦略

### 8.1 テスト方針

| レベル | カバレッジ目標 | 使用フレームワーク |
|---|---|---|
| Unit / Component | 各 AC に 1 件以上（数値目標は未設定） | vitest + Testing Library + msw |
| Integration | API 契約は schema.d.ts の型で検証 | tsc --noEmit |
| E2E | 予約→マイページ→キャンセル、二重予約、IDOR | Playwright（scripts/e2e.sh） |

### 8.2 テスト観点

- [x] 正常系: 枠表示・医師絞り込み・予約確定・一覧・キャンセル・日時変更
- [x] 異常系: 409（満席・重複・変更先満席）、422（期限超過）、取得失敗の再試行
- [x] 境界値: 症状メモ 1000 文字、今日より前の日付、枠なしの日、JST 表示
- [x] セキュリティ: 未ログインは予約フォームを出さない、他人の予約を API で見られない（E2E）

### 8.3 テスト除外事項
Tailwind の見た目（色・余白）は自動テストしない。

### 8.4 テストデータ方針
単体は msw、E2E は scripts/e2e.sh が起動する実 PostgreSQL に、テストごとに別アカウントを作る。

---

## 9. Definition of Done

- [x] 全テストケースが通過している（vitest 222 件、Playwright 22 件）
- [x] 型エラーが 0 件である
- [x] 既存テストが破壊されていない
- [ ] 禁止事項（セクション 7）を全て遵守している（drift_check で確認）
- [ ] IMP の受け入れ基準（EARS）を全て満たしている（drift_check で確認）
- [ ] コードレビュー指摘事項が解消されている（PR #18）

---

## 10. ロールバック計画

### 10.1 ロールバックトリガー条件
予約・キャンセルが本番で失敗する、または他の画面を巻き込む不具合。

### 10.2 ロールバック手順
1. PR #18 のコミットを revert（フロントのみ。バックエンドの API は後方互換）
2. フロントを再ビルド・再配信

### 10.3 ロールバック所要時間
- 自動: —
- 手動: 15 分程度（ビルド + 配信）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| 並行予約で満席 | M | H | 409 を受けて枠を再取得し選び直せる |
| 症状メモの漏えい | H | L | 本人のみ表示、ログ・ストレージに残さない |
| キャンセル期限の判定がクライアントとサーバーで食い違う | M | L | 表示は API の cancelable、最終判定はサーバー（422） |
| 診療科を名前から引く（RescheduleDialog） | L | L | 名前が一意の前提。重複した場合は API に dept_id を追加 |

---

## 12. レビュアーチェックリスト

IMP-checklist.md を参照。

---

## 13. 承認記録

| ロール | 担当者 | 承認日時 | コメント |
|---|---|---|---|
| アーキテクト | | | |
| セキュリティ | | | |
| QA | | | |
| 実装者 | | | |

---

## Appendix: 変更履歴

| バージョン | 変更内容 | 変更者 | 日時 |
|---|---|---|---|
| 1.0.0 | 実装（PR #18）から後追いで作成 | Claude | 2026-10-10T20:10:00+09:00 |
| 1.0.1 | drift_check の指摘を反映（visit_type を revisit に訂正、position API を追記） | Claude | 2026-10-10T20:40:00+09:00 |
