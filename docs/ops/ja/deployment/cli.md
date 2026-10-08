---
title: "コマンドライン引数"
layout: doc
outline: deep
lastUpdated: true
---

# コマンドライン引数

1. `--port <port_number>`: サーバーが待ち受けるポート番号を指定します。デフォルトは `3000` です。
   - 例：`--port 3000`
2. `--log-dir <log_dir>`: ログディレクトリを指定します。未指定の場合は、作業ディレクトリ配下の `logs` フォルダーに保存されます。
   - 例：`--log-dir ./logs`
3. `--version`: システムのバージョン番号を出力して終了します。
4. `--help`: コマンドの使い方と引数の説明を表示します。
5. `--config <config_file>`: 設定ファイルのパスを指定します。プログラムはそのファイルの設定を読み込みます。設定ファイルの完全なキー名とデフォルト値は、配布パッケージのルートにある `config.example.yaml` を参照してください。
   - 例：`--config ./config.yaml`
