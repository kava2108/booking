# Red フェーズ: 3-user-auth-go-infra / 1.3

実装前に `go test ./...` → 依存・実装が無くビルド不能で全パッケージ失敗（期待どおり）。

| テスト | ケース |
|---|---|
| config | 既定値 / 上書き / 必須 3 項目欠落（キー名のみ表示・値は非表示）/ 不正 duration / RATE_LIMIT 0 |
| request_id | 生成 / 引継ぎ / 長すぎ・改行・空白は再生成 / 未設定時は空文字 |
| logger | request_id・method・path・status・latency 出力 / ボディ・Authorization・Cookie・クエリ文字列を出さない / 5xx=error, 4xx=warn / ハンドラー error の最終ステータス |
| rate_limiter | 5 回通過・6 回目 429 + Retry-After / IP 別 / 期間経過後に解除 / X-Forwarded-For 偽装で迂回できない / ストア障害で 503 |
| main | /healthz 200 + X-Request-ID / ログに request_id / XFF を信用しない / 404 |
