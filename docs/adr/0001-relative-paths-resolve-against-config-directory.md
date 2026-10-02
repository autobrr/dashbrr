# Relative paths in the config file resolve against the config directory

A relative `[database] path` in `config.toml` is relative to the config directory, not to the working directory of the process. Before this decision, `serve` ignored the value in the file and always used `<config directory>/data/dashbrr.db`. With this rule, the default value `./data/dashbrr.db` gives the same file, so existing setups keep their database. A relative path from `--db-file` or `DASHBRR__DB_PATH` stays relative to the working directory, because a person types those values in a shell.
