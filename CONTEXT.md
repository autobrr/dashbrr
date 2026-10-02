# dashbrr

dashbrr is a self-hosted dashboard that shows the health and status of media services. This glossary names the concepts that its configuration and code use.

## Language

### Configuration

**Config file**:
The one `config.toml` file that a dashbrr process reads at startup. The `serve` command and the CLI commands select it in the same way.
_Avoid_: config path (when you mean the file itself), settings file

**Config directory**:
The directory that contains the config file. A relative path in the config file is relative to this directory.
_Avoid_: data directory, working directory

**Database path**:
The location of the SQLite database file. The default is `data/dashbrr.db` in the config directory.
_Avoid_: db file, data path

### Service data

**Poller**:
The one component that fetches data from services and publishes it to the UI.
_Avoid_: SSE producer, fetcher

**Detail job**:
One scheduled fetch of one kind of data for one service instance.
_Avoid_: stats job, poll task

**Service payload**:
The data for one service instance that the UI receives over SSE.
_Avoid_: stats, service data
