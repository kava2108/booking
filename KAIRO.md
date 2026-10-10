# KAIRO — AI-TDD Engine for booking

## 標準ワークフロー

```bash
# 1. Issue からタスク構造を起こす
/kairo:issue_init GH-123

# 2. IMP（実装管理計画書）を生成する
/kairo:imp_generate GH-123

# 3. 実装する（TDD モード）
/kairo:implement GH-123 --mode tdd

# 4. テストを生成・実行する
/kairo:test GH-123 --exec

# 5. 逆仕様を生成する
/kairo:rev GH-123

# 6. 乖離を確認する
/kairo:drift_check GH-123

# 7. 全体を同期する
/kairo:sync GH-123

# 8. レビュー資料を生成する
/kairo:review GH-123
```

## よく使うコマンド

```bash
# 自然言語から開始する
/kairo:cli [やりたいことを自然言語で入力]

# ヘルプ
/kairo:help
```

## プロジェクト設定

設定ファイル: `.kairo/config.json`

## 注意事項

- すべてのコマンドは冪等（再実行安全）です
- IMP.md が全フェーズの単一の真実の源です
- drift スコアが 20 を超えたら IMP または実装の修正が必要です
