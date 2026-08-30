# Direct torrent client integrations

Dashbrr will not add direct integrations for torrent clients (qBittorrent, Deluge, Transmission). Torrent activity comes from qui, which dashbrr already supports.

## Why this is out of scope

Qui is a qBittorrent web interface with an API key. Dashbrr polls its `/api/qui/overview` endpoint and shows transfer counts, speeds, and totals. This covers the qBittorrent case without a new credential type.

A direct qBittorrent integration would need cookie-based session authentication with a user name and a password, which no other dashbrr service uses. Deluge and Transmission each need their own protocol and their own card. Each new client also adds a poll job, a health check, and frontend components. That cost is not justified while qui gives the same view.

This decision can change. If people want Deluge or Transmission, open a new issue with the specific client.

## Prior requests

- [#62: Add support for torrent clients](https://github.com/autobrr/dashbrr/issues/62)
