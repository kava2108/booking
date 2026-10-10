# Red フェーズ: 5-clinic-reservation-sqlc-repository / 1.2

テスト: `internal/repository/db/clinic_queries_test.go`（39 件の統合テスト）、`internal/repository/store_test.go`、`mocks_test.go`

クエリ・interface 未作成の状態 → `go vet` が `undefined`（CreateSlotIfAbsent ほか）で失敗（期待どおり）。

| 観点 | ケース |
|---|---|
| 枠 | 医師・期間での絞込 / 既存枠を上書きしない / 満席で ErrNoRows / 休診枠は確保不可 / **20 並行で capacity=1 に成功 1 件** / 解放が負数にならない / 休診で範囲内のみ closed |
| 予約 | 一覧は自分のみ・日時昇順 / 二重予約は 23505 / 同時刻の重複検出 / **他人の予約は取得・キャンセル・変更不可（IDOR）** / キャンセル後の再予約 / 通知対象の抽出 |
| 待機列 | 日次採番 / 採番衝突は 23505 / **不正な状態遷移は ErrNoRows** / 二重呼出・二重キャンセル不可 / CountAhead（診察中は数え、完了・キャンセルは数えない）/ 直近所要時間 / 診療時間内の医師数（開始・終了境界、休診、非アクティブ）/ スタッフ一覧に氏名・症状 / 患者本人の順番参照は IDOR 対策 |
| 通知 | リース / 送信済み除外 / 未来の send_after 除外 / リース切れ再取得で attempts 加算 / failed 除外 / **6 ワーカー同時取得で重複なし** |
| 管理 | 設定更新と CHECK / 診療科設定 / スケジュール差替え / 非アクティブ医師の除外 / 監査ログ |
| Tx | commit / 予約失敗時に枠確保も rollback / panic で rollback + 再 panic / ErrNoRows の透過 |
