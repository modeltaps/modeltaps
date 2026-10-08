---
title: "CLI Options"
layout: doc
outline: deep
lastUpdated: true
---

# CLI Options

1. `--port <port_number>`: The port the server listens on. Defaults to `3000`.
   - Example: `--port 3000`
2. `--log-dir <log_dir>`: The log directory. If unset, logs are written to the `logs` folder under the working directory.
   - Example: `--log-dir ./logs`
3. `--version`: Print the system version and exit.
4. `--help`: Show command usage and option descriptions.
5. `--config <config_file>`: Path to a configuration file. The program reads its configuration from this file. For the full list of keys and their default values, refer to `config.example.yaml` in the root of the delivery package.
   - Example: `--config ./config.yaml`
