---
issue_id: 17-clinic-reservation-frontend-booking
run_id: drift-20261010-001
run_at: 2026-10-10T20:25:00+09:00
imp_version: 1.0.0
drift_baseline: e67336bf4d6f20eada9cbf37744f4b5d2eb4027b
drift_score: 13
threshold: 20
status: Minor
---

# 乖離レポート: 17-clinic-reservation-frontend-booking

## スコアサマリー

```
drift スコア: 13/100 — Minor Drift

CRITICAL: 0件 (×10点 = 0点)
WARNING:  3件 (×3点  = 9点)
INFO:     4件 (×1点  = 4点)
合計:     13点
```

> 注: 本 IMP は実装の後に作成した。乖離の多くは「IMP の記述誤り」であり、実装側の欠陥ではない。

---

## D1: 機能仕様の乖離

| AC | テスト | 実装 | 判定 | スコア |
|---|---|---|---|---|
| REQ-001-AC-1 枠を時刻順に表示 | ✅ SlotPicker.test「空き枠を時刻順に表示」 | ✅ SlotPicker（順序は API 依存） | COVERED | 0 |
| REQ-001-AC-2 医師指名で絞り込み | ✅ BookingFlow.test「医師を指名すると…」 | ✅ BookingFlow → useSlots(doctor_id) | COVERED | 0 |
| REQ-001-AC-3 満席・休診・締切後は選択不可 | ✅ SlotPicker.test ほか、E2E「休診」 | ✅ REASON_LABEL + disabled | COVERED | 0 |
| REQ-002-AC-1 予約確定・予約番号 | ✅ BookingFlow.test、E2E「新規登録→予約」 | ✅ BookingDialog | COVERED | 0 |
| REQ-003-AC-1 予約一覧（今後・過去） | ✅ AppointmentList.test | ✅ AppointmentList | COVERED | 0 |
| REQ-003-AC-2 期限内キャンセル | ✅ AppointmentList.test、E2E | ✅ CancelDialog（枠解放・通知はサーバー側 #12 / #14） | COVERED | 0 |
| REQ-003-AC-3 期限超過の案内 | ✅ AppointmentList.test | ⚠️ 「医療機関へお電話ください」のみ。電話番号など具体的な連絡先が画面のどこにも無い | COVERED（注意） | +1 INFO |
| REQ-003-AC-4 日時変更 | ✅ AppointmentList.test（成功・変更先満席） | ✅ RescheduleDialog（原子性はサーバー側 #12） | COVERED | 0 |

**D1 小計**: 1点

---

## D2: API 契約の乖離

| エンドポイント | IMP 仕様 | 実装 | 乖離内容 | 判定 | スコア |
|---|---|---|---|---|---|
| GET /departments, /doctors, /slots | 記載あり | 一致 | — | OK | 0 |
| POST /appointments | visit_type: "first" \| **"follow_up"** | openapi.yaml・実装とも **"revisit"** | IMP の記述誤り（IMP 4 の予約作成・TASK-0002） | WARNING | +3 |
| GET /appointments/me, PATCH / DELETE /appointments/{id} | 記載あり | 一致 | — | OK | 0 |
| GET /appointments/{id}/position | 未記載 | AppointmentCard 内の PositionPanel（features/waittime）が呼ぶ | IMP 未記載の利用 | WARNING | +3 |

**D2 小計**: 6点

---

## D3: スキーマの乖離

| スキーマ変更 | IMP 仕様 | 実装 | 乖離内容 | 判定 | スコア |
|---|---|---|---|---|---|
| なし | 変更なし | 変更なし | — | OK | 0 |

**D3 小計**: 0点

---

## D4: テストカバレッジの乖離

| テスト種別 | IMP 目標 | 実際 | 乖離 | 判定 | スコア |
|---|---|---|---|---|---|
| Unit / Component | 各 AC に 1 件以上（数値目標なし） | 8 AC すべてに対応するテストあり（vitest 222 件成功） | なし | OK | 0 |
| E2E | 予約→マイページ→キャンセル、二重予約、IDOR | Playwright 22 件成功（patient.spec に3件とも存在） | なし | OK | 0 |
| セキュリティ | 未ログイン、IDOR | 実装あり | なし | OK | 0 |

- 数値のカバレッジは未計測（目標も未設定のため乖離としては扱わない）。
- 日時変更（REQ-003-AC-4）は単体テストのみで、E2E は無い。

**D4 小計**: 0点

---

## D5: タスク完了状態の乖離

| タスク | 状態 | 判定 | スコア |
|---|---|---|---|
| TASK-0001 空き枠の検索 | 実装・テスト済み / patch-plan なし | INFO | +1 |
| TASK-0002 予約の確定 | 実装・テスト済み / patch-plan なし | INFO | +1 |
| TASK-0003 マイページ | 実装・テスト済み / patch-plan なし | INFO | +1 |

IMP 未記載のタスクの実装: なし。

**D5 小計**: 3点

---

## D6: デザインシステム乖離

| 項目 | 結果 | 判定 | スコア |
|---|---|---|---|
| 16 進カラー直書き・font-family・font-size 直指定 | 該当なし | OK | 0 |
| MASTER.md にない色の濃淡 | BookingDialog.tsx:76, 81 の text-red-600 / text-red-700（MASTER.md の定義は error-500 = red-500 のみ） | WARNING | +3 |
| Pre-delivery Checklist（U01–U06）/ Anti-patterns | MASTER.md にこれらの節が無く、判定できない | N/A | 0 |

**D6 小計**: 3点

---

## 推奨アクション

### CRITICAL（即時対応）
- なし

### WARNING（早期対応）
- IMP の visit_type を "revisit" に直す → `/kairo:imp_generate 17-clinic-reservation-frontend-booking --update`（パッチ版）
- IMP の 3.2 に GET /appointments/{id}/position（PositionPanel 経由）を追記する → 同上
- BookingDialog のエラー文字色を MASTER.md のトークン（error-500）に揃えるか、MASTER.md に text-red-600 / 700 を追記する

### INFO（任意対応）
- 期限超過時に表示する医療機関の連絡先（電話番号）を持つか決める。API や設定に連絡先が無いため、AC 文言の「連絡方法」を「電話で連絡」と解釈している
- patch-plan は実装後の作成になるため不要（implements/ は作らない）
- MASTER.md に U01–U06 と Anti-patterns の節を足すと、D6 を判定できる

---

## 前回との比較

| 指標 | 前回 | 今回 | 変化 |
|---|---|---|---|
| drift スコア | — | 13 | 初回 |
| CRITICAL | — | 0 | |
| WARNING | — | 3 | |
