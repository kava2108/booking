# Red フェーズ: 11-clinic-reservation-slot-api-admin-settings / 2.1

service / HTTP / DB のテストを先に書き、`undefined`（SlotService, AdminService, Deps.Slots ほか）でビルドが失敗することを確認。

| ケース | AC |
|---|---|
| 枠検索: 時刻順、full / closed / past_cutoff / outside_window の判定と優先順位（closed > past > window > full）、現在ちょうどの枠は締切後 | REQ-001-AC-1/3 |
| 受付期間の境界（最終日は可・翌日は outside_window）、医師指名で絞込、未知の診療科 422、空は `[]` | REQ-001-AC-2/3 |
| 枠生成: 月曜 9-12 の 30 分 = 6 枠、終了ちょうど開始の枠は作らない、割り切れない枠長、曜日の対応、**既存の休診を再適用**、範囲検証 | REQ-006-AC-1 |
| 管理者: スケジュール置換（隣接は可・重複は不可・曜日/時刻/枠長/定員の検証）、検証エラー時は既存を変更しない、未知の医師 | REQ-006-AC-1 |
| 設定: 部分更新で他の値を維持、診療科設定（省略 = NULL）、範囲の境界、**未知の診療科でロールバック（原子性）** | REQ-006-AC-2 |
| HTTP: 認証なしで公開 API / 公開レートリミット 429 / 内部情報（定員・予約数）を返さない / 冪等な再保存 / 予約済み枠を壊さない / 認可マトリクス（患者・スタッフ・システム管理者は 403）/ **OpenAPI 契約チェック** | REQ-001, 006 |
| DB: GetDepartment / GetDoctor / ListClosuresFrom | — |
