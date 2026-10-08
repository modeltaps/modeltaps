---
title: "命令列參數"
layout: doc
outline: deep
lastUpdated: true
---

# 命令列參數

1. `--port <port_number>`: 指定伺服器監聽的端口號，預設為 `3000`。
   - 例子：`--port 3000`
2. `--log-dir <log_dir>`: 指定日誌檔案夾，如果沒有設定，預設儲存至工作目錄的 `logs` 檔案夾下。
   - 例子：`--log-dir ./logs`
3. `--version`: 打印系統版本號並退出。
4. `--help`: 查看命令的使用說明和參數說明。
5. `--config <config_file>`: 指定設定檔路徑，程式將會讀取該檔案中的配置。設定檔的完整鍵名與預設值以交付包根目錄的 `config.example.yaml` 為準。
   - 例子：`--config ./config.yaml`
