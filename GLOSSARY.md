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

**URL**:
The address that dashbrr uses to reach a service. It must be an absolute `http://` or `https://` address with a host name.
_Avoid_: service address, endpoint

**Base path**:
The URL path prefix that dashbrr is served under, for example `/dashbrr`. With no base path, dashbrr is served at the root of the host.
_Avoid_: base URL, sub-path, URL prefix

**Access URL**:
The address that the user's browser uses to open a service. It is optional. When it is set, links to the service use the access URL. Else they use the URL.
_Avoid_: external URL, public URL

**Discovered service**:
A service whose record a discovery source owns. The annotations on its Kubernetes Service set its fields. When the annotations have no URL, the ports of the Service set the URL. When the annotations have no access URL, the first hostname of an HTTPRoute to the Service sets the access URL. A sync adds, changes, and deletes the service to agree with the Kubernetes Service. A service that the user adds in the UI or imports from a file is not a discovered service.
_Avoid_: auto service, synced service

### Authentication

**Session**:
The record of a logged-in user that the server keeps after a builtin or OIDC login. It ends at logout or after 30 days.
_Avoid_: login, auth session

**Session token**:
The random string that identifies a session. The browser sends it in the `dashbrr_user_session` cookie. API clients send it as a Bearer token.
_Avoid_: session ID, access token

**Login type**:
How a session was created: builtin (username and password) or OIDC. The API field is `auth_type`.
_Avoid_: auth type, auth method

**Logout**:
Ending the user's session in dashbrr. It is the same for both login types. It does not end the user's session at the OIDC provider.
_Avoid_: sign out, provider logout

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

### *arr apps

***arr app**:
A service with a download queue that dashbrr shows and can remove items from: Sonarr, Radarr, Lidarr, Readarr, or Whisparr. Prowlarr and Bazarr are not *arr apps, because they have no download queue.
_Avoid_: arr service, Starr app

**Queue item**:
One entry in the download queue of an *arr app.
_Avoid_: queue record
