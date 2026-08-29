# Plugin system

Dashbrr will not support runtime-installable plugins. Add integrations to the main codebase when they need service-specific behavior.

## Why this is out of scope

Each integration can include health checks, polling jobs, credentials, API routes, data payloads, and React components. A plugin system would need stable contracts for all of these, plus packaging and compatibility rules. That maintenance cost is not justified without a concrete integration that cannot fit the existing model.

Dashbrr ships as a single Go binary with an embedded frontend. Loading third-party backend or frontend code at runtime would also complicate that deployment model.

For simple custom integrations, the General Service polls an arbitrary URL with an optional bearer token. It reads `status` and `message` fields and displays up to 12 other top-level scalar JSON fields on the service card.

## Prior requests

- [#31: Integrations/Plug-In system](https://github.com/autobrr/dashbrr/issues/31)
