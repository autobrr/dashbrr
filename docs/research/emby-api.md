# Emby API research for issue #212

Date: 2026-10-12. Scope: the Emby Server endpoints, headers, and fields that the dashbrr Jellyfin client uses.

## Summary

Yes. dashbrr can reuse most of the Jellyfin client for Emby. The Emby client needs a different service type, a different auth header, an `/emby` path prefix, and small changes for three missing fields.

- Auth header. Send `X-Emby-Token: <key>`. Emby documents this header and the `api_key` query parameter for API keys. Emby does not document `Authorization: MediaBrowser Token="..."`, which is the header that dashbrr sends to Jellyfin now. The last open-source Emby code (2018) accepted that header. Current Emby is closed source, so no primary source verifies that it still does.
- Paths. The paths are the same: `/System/Info` and `/Sessions`. Emby documents URLs as `http://host:8096/emby/{path}`. The 2018 source registers each route with and without the `/emby` prefix. Not verified for current Emby.
- Missing fields. Emby `SystemInfo` has no `ProductName`. Emby `SessionInfo` has no `IsActive`. Emby `GET /Sessions` documents no `ActiveWithinSeconds` query parameter. All other fields that dashbrr reads have the same name and a compatible type.
- New field. Emby `SystemInfo` has `HasUpdateAvailable`. dashbrr can use it for the update badge. Jellyfin has no such field.
- Version format. Emby uses four-part version numbers, for example `4.10.1.0`. Jellyfin uses a shorter form. The current Jellyfin release tag is `v12.2`.

## Sources used

The primary sources are:

- Emby SDK. Emby generates the developer docs at dev.emby.media from the `MediaBrowser/Emby.SDK` repository. That repository also holds a Swagger 2.0 spec, `Documentation/Download/openapi_v2_noversion.json`. This report used commit `1faf8176df`, "SDK Version 4.10.1.0", dated 2026-09-29. Below, this file is "the spec".
- Emby API clients. `MediaBrowser/Emby.ApiClients` holds clients that Emby generates from the spec. This report used commit `3f25bdbd17`.
- Old Emby server source. `MediaBrowser/Emby` holds the last open-source server code. Its last commit is `1d7c2ab4bf`, dated 2018-09-20. Emby Server 4.8 and 4.9 are closed source. This code shows how Emby behaved in 2018. It does not prove how current Emby behaves.
- Jellyfin source. `jellyfin/jellyfin` at tag `v12.2` (commit `ca0f16eb7c`).

The generated API browser at dev.emby.media does not name an Emby server version. The SDK commit message names 4.10.1.0, so the spec is at least as new as Emby 4.8 and 4.9.

## What dashbrr uses from Jellyfin today

These facts come from `internal/services/jellyfin/jellyfin.go`, `internal/types/jellyfin.go`, `internal/api/handlers/service_payload_builders.go`, and `web/src/components/services/jellyfin/JellyfinStats.tsx`:

- Requests: `GET {url}/System/Info` and `GET {url}/Sessions?ActiveWithinSeconds=300`.
- Headers: `Accept: application/json` and `Authorization: MediaBrowser Token="<key>"`.
- Session filter: the client keeps only the sessions that have a `NowPlayingItem` with a `Name` or a `SeriesName`.
- Default URL: `http://localhost:8096`.
- System fields: `ServerName`, `Version`, `ProductName`, and `Id`. The UI shows only `ServerName` and `Version`. dashbrr decodes `ProductName`, but no code reads it.
- Session fields: see the table near the end of this file.

## 1. Auth methods for an API key

| Method | Documented by Emby today? | Accepted by the 2018 source? |
|---|---|---|
| `X-Emby-Token: <key>` header | Yes. It is the first method on the API key page, and the spec names it. | Yes |
| `api_key=<key>` query parameter | Yes. The docs call it "an even easier method". | Yes |
| `X-Emby-Authorization: Emby ... Token="<key>"` | Yes, for the user login flow. The spec shows the `Emby` scheme. | Yes. The code accepts the `Emby` and `MediaBrowser` schemes. |
| `Authorization: Emby ... Token="<key>"` | Yes, in the same way. The spec says the header can be named `Authorization` or `X-Emby-Authorization`. | Yes |
| `Authorization: MediaBrowser Token="<key>"` | No. The spec shows only the `Emby` scheme. | Yes |
| `X-MediaBrowser-Token` header | No | Yes |

The evidence for each row:

- API key page. The page says "Send the API key string in an http request heaser [sic] named X-Emby-Token", and "An even easier method is to transmit the API key as a query string parameter name api_key". Its example is `http://localhost:8096/emby/System/Info?api_key=...`. Sources: https://dev.emby.media/doc/restapi/API-Key-Authentication.html, and the same page as `Documentation/doc/restapi/API-Key-Authentication.html` in Emby.SDK at `1faf8176df`.
- Spec security definitions. The spec defines `securityDefinitions.apikeyauth` as `{"type":"apiKey","name":"api_key","in":"query"}`. The description adds "The api key can alternatively be specified in an http header named _X-Emby-Token_". Source: `openapi_v2_noversion.json` in Emby.SDK at `1faf8176df`.
- Spec login header. In the spec, the `/Users/AuthenticateByName` operation has a header parameter named `X-Emby-Authorization`. Its description says "The authorization header can be either named 'Authorization' or 'X-Emby-Authorization'. It must be of the following schema: Emby UserId="(guid)", Client="(string)", Device="(string)", DeviceId="(string)", Version="string", Token="(string)"". Source: the same file.
- User login page. This page sends `Authorization: Emby UserId=..., Client=..., Device=..., DeviceId=..., Version=...` to identify the client. After login, the client sends the access token in `X-Emby-Token`. Source: https://dev.emby.media/doc/restapi/User-Authentication.html.
- Official generated client. The official C# client sends only `X-Emby-Token` for an API key, with `new HeaderParameter("X-Emby-Token", this.ApiKey)`. Source: `Clients/Emby.ApiClient/Emby.ApiClient/Client/Authentication/EmbyApiKeyAuthenticator.cs` in Emby.ApiClients at `3f25bdbd17`.
- 2018 server code. The code reads `X-Emby-Authorization` first, then `Authorization`. It accepts the `MediaBrowser` and `Emby` schemes and reads `Token=` from the header. If it finds no token there, it tries `X-Emby-Token`, then `X-MediaBrowser-Token`, then the `api_key` query parameter. Source: `Emby.Server.Implementations/HttpServer/Security/AuthorizationContext.cs` in MediaBrowser/Emby at `1d7c2ab4bf`.

Preferred method. Emby does not use the word "preferred". The API key page names `X-Emby-Token` first, the spec names it, and the official client sends it. A query parameter puts the key in proxy logs, so use the header. Jellyfin disabled its legacy auth methods by default in jellyfin/jellyfin PR #15559 (merged 2025-11-27). For this reason, the Emby client needs its own header code and cannot share the Jellyfin header.

Not verified. No primary source shows that current Emby (4.8, 4.9, or 4.10) still accepts `Authorization: MediaBrowser Token="..."`. No primary source shows that current Emby accepts an `Authorization: Emby Token="..."` header without the `Client`, `Device`, `DeviceId`, and `Version` fields.

## 2. `/System/Info`

- Path. The path is `GET /System/Info`. The spec sets `"x-original-basePath": "/emby"`. Question 4 covers the prefix.
- Auth. The spec says "Requires authentication as user", and allows `apikeyauth` or `embyauth`. An API key works, and the key does not need an admin user. In the 2018 source, the route has `[Authenticated(EscapeParentalControl = true, AllowBeforeStartupWizard = true)]` and no admin role. Sources: https://dev.emby.media/reference/RestAPI/SystemService/getSystemInfo.html and `MediaBrowser.Api/System/SystemService.cs` at `1d7c2ab4bf`.
- Response. The response is a `SystemInfo` object. Its fields include `ServerName`, `Version`, `Id`, `HasUpdateAvailable`, `OperatingSystem`, `OperatingSystemDisplayName`, `PackageName`, `HasPendingRestart`, `IsShuttingDown`, `IsInMaintenanceMode`, `HardwareAccelerationRequiresPremiere`, `SystemUpdateLevel`, `HttpServerPortNumber`, `HttpsPortNumber`, `SupportsHttps`, `WebSocketPortNumber`, `LocalAddress`, `LocalAddresses`, `WanAddress`, `RemoteAddresses`, `CanSelfRestart`, and `CanSelfUpdate`. `SystemUpdateLevel` is an enum with the values `Release`, `Beta`, and `Dev`. Source: the spec, `definitions.SystemInfo`.
- Comparison with Jellyfin. `ServerName`, `Version`, and `Id` exist in both servers as strings. Emby has no `ProductName`. Jellyfin has it on `PublicSystemInfo` (`MediaBrowser.Model/System/PublicSystemInfo.cs` at `v12.2`). dashbrr does not show `ProductName`, so the missing field has no effect.

`/System/Info/Public` exists. Its response is `PublicSystemInfo`, with the fields `LocalAddress`, `LocalAddresses`, `WanAddress`, `RemoteAddresses`, `ServerName`, `Version`, `Id`, and `SDK`. Source: https://dev.emby.media/reference/RestAPI/SystemService/getSystemInfoPublic.html.

The generated page for this endpoint also says "Requires authentication as user". The same text appears on every operation, so it is probably a default of the doc generator. In the 2018 source, the `/System/Info/Public` route has no `[Authenticated]` attribute, so anyone could call it (`SystemService.cs` at `1d7c2ab4bf`). Not verified for current Emby. This does not affect dashbrr, because dashbrr needs a valid key anyway and uses `/System/Info` for the health check.

## 3. `/Sessions`

- Path and auth. The path is `GET /Sessions`, and the docs say "Requires authentication as user". Source: https://dev.emby.media/reference/RestAPI/SessionsService/getSessions.html.
- Query parameters. The spec documents `ControllableByUserId`, `DeviceId`, and `Id`. It does not document `ActiveWithinSeconds`. Jellyfin has `activeWithinSeconds` (`Jellyfin.Api/Controllers/SessionController.cs` at `v12.2`). Not verified: whether Emby ignores an unknown query parameter or rejects it. The safe choice is to omit the parameter for Emby. dashbrr already drops sessions that have no `NowPlayingItem`, so the result changes very little.
- Response. The response is an array of `Session.SessionInfo`. Source: the spec, `definitions["Session.SessionInfo"]`.
- Fields. Every session field that dashbrr reads exists, except `IsActive`. The table below lists each field.

Effect of the missing `IsActive`. `JellyfinStats.tsx` counts a session as playing when it is not paused, it has a `NowPlayingItem`, and either `IsActive` is true or `PositionTicks` is more than 0. Without `IsActive`, a stream at position 0 that is not paused does not count as playing until its position moves. The effect is small.

`MediaStreams` on `NowPlayingItem`. The spec lists `MediaStreams` on `BaseItemDto`, but it does not say whether `/Sessions` fills in that field. The 2018 `SessionManager.GetItemInfo` removes `ItemFields.MediaStreams` from the item fields. It then sets `info.MediaStreams = mediaSource.MediaStreams` from the media source that plays (`Emby.Server.Implementations/Session/SessionManager.cs` at `1d7c2ab4bf`). So in 2018, the field had data during playback. Not verified for current Emby.

`TranscodingInfo`. The Emby object has all the fields that dashbrr reads. It also has fields that dashbrr does not read, for example `VideoDecoder`, `VideoEncoder`, `VideoDecoderIsHardware`, `VideoEncoderIsHardware`, `AudioBitrate`, `VideoBitrate`, `Framerate`, and `CurrentThrottle`. `TranscodeReasons` is an array of enum strings, for example `VideoCodecNotSupported` and `ContainerBitrateExceedsLimit`. dashbrr does not read it today.

`PlayMethod`. This is an enum with the values `Transcode`, `DirectStream`, and `DirectPlay`. Jellyfin uses the same values. dashbrr compares the value to "transcode" without regard to case, so the comparison works.

## 4. Base path (`/emby`)

The Emby REST docs give the URL form as `http[s]://hostname:port/emby/{apipath}`. Every example uses `/emby`, and the spec sets `"x-original-basePath": "/emby"`. The generated Go client sets `BasePath: "http://emby.media/emby"` by default (`Clients/Go/configuration.go` in Emby.ApiClients at `3f25bdbd17`). Sources: https://dev.emby.media/doc/restapi/index.html and https://dev.emby.media/doc/restapi/API-Key-Authentication.html.

In 2018, `HttpListenerHost.GetRouteAttributes` registered each route four times: as `/X`, as `/emby/X`, as `/mediabrowser/X`, and as `/emby/emby/X`. A code comment says "needed because apps add /emby, and some users also add /emby, thereby double prefixing". Source: `Emby.Server.Implementations/HttpServer/HttpListenerHost.cs` at `1d7c2ab4bf`. Not verified: whether the root path without `/emby` still works on Emby 4.8 or 4.9.

Effect on the URL that the user enters:

- Keep the URL that the user enters as the base, for example `http://host:8096`.
- Add `/emby` before each API path in code, for example `{url}/emby/System/Info`. This follows the documented form.
- If the user already typed `/emby` at the end of the URL, the request goes to `/emby/emby/System/Info`. The 2018 source accepted that path, but current Emby is not verified. To avoid the question, remove a trailing `/emby` from the URL before you add the prefix.
- If Emby runs behind a reverse proxy under a sub-path, the user enters that sub-path, and `/emby` goes after it.

## 5. Default port and version format

- Ports. The Emby connectivity article says to open TCP 8096, and TCP 8920 for HTTPS if HTTPS is on. UDP 7359 is for local server discovery. The API key example uses `localhost:8096`. So the default URL `http://localhost:8096` is the same as for Jellyfin. Sources: https://emby.media/support/articles/Connectivity.html and https://dev.emby.media/doc/restapi/API-Key-Authentication.html.
- Version string. The Emby SDK commits use four-part numbers: "SDK Version 4.9.3.0", "4.9.5.0", "4.10.0.40", and "4.10.1.0" (Emby.SDK commit log). Not verified against the `Version` value from a live server. dashbrr only shows and caches the string, so the format has no effect today. If someone adds a semver comparison later, it must accept four parts.
- Update flag. `SystemInfo.HasUpdateAvailable` is a boolean. The Jellyfin client in dashbrr never sets the update status, so `updateAvailable` is always false for Jellyfin. An Emby client can store `HasUpdateAvailable` with `CacheUpdateStatus`.
- Emby Premiere. `SystemInfo` has `HardwareAccelerationRequiresPremiere` (boolean). The spec has no other Premiere or license field on `SystemInfo` or `SessionInfo`. The spec shows no Premiere requirement on the endpoints that dashbrr uses.

## 6. Other differences that affect reuse

- Auth header: the client must send `X-Emby-Token` for Emby. See question 1.
- Path prefix: the client must add `/emby` for Emby. See question 4.
- `ActiveWithinSeconds`: the spec does not document it. Omit it for Emby, or make sure on a real server that Emby ignores it.
- `IsActive` and `ProductName`: both are missing. The Go types decode them to zero values without an error. The UI changes a little. See question 3.
- Integer sizes: Emby uses int32 for `TranscodingInfo.Bitrate`, `MediaStream.BitRate`, `Width`, `Height`, `Channels`, and `AudioChannels`. dashbrr uses `int64` and `int`, which hold all int32 values.
- Response format: the spec lists `application/json` and `application/xml`. dashbrr sends `Accept: application/json`, so Emby returns JSON.
- Names in errors: the error type and the log messages say "jellyfin". A shared client must take the service name as a parameter.

This report found no other differences that block reuse. Both servers use PascalCase JSON field names.

## Field-by-field comparison

"Same" means the same name and a compatible type. The Emby types come from the spec, `openapi_v2_noversion.json` at Emby.SDK `1faf8176df`.

| dashbrr field (Jellyfin) | dashbrr Go type | Emby field | Emby type | Status |
|---|---|---|---|---|
| SystemInfo.ServerName | string | ServerName | string | Same |
| SystemInfo.Version | string | Version | string | Same. Emby uses four parts. |
| SystemInfo.ProductName | string | None | None | Missing. The UI does not show it. |
| Not used | None | HasUpdateAvailable | boolean | Emby only. It can drive the update badge. |
| SystemInfo.Id | string | Id | string | Same |
| Session.Id | string | Id | string | Same |
| Session.UserName | string | UserName | string | Same |
| Session.Client | string | Client | string | Same |
| Session.DeviceName | string | DeviceName | string | Same |
| Session.DeviceType | string | DeviceType | string | Same |
| Session.ApplicationVersion | string | ApplicationVersion | string | Same |
| Session.IsActive | bool | None | None | Missing. The UI falls back to `PositionTicks > 0`. |
| Session.PlayState | object | PlayState | PlayerStateInfo | Same |
| PlayState.PositionTicks | int64 | PositionTicks | int64 | Same |
| PlayState.IsPaused | bool | IsPaused | boolean | Same |
| PlayState.PlayMethod | string | PlayMethod | enum `Transcode`, `DirectStream`, `DirectPlay` | Same |
| PlayState.AudioStreamIndex | *int | AudioStreamIndex | int32 | Same |
| Session.NowPlayingItem | object | NowPlayingItem | BaseItemDto | Same |
| NowPlayingItem.Name | string | Name | string | Same |
| NowPlayingItem.SeriesName | string | SeriesName | string | Same |
| NowPlayingItem.Type | string | Type | string | Same |
| NowPlayingItem.RunTimeTicks | int64 | RunTimeTicks | int64 | Same |
| NowPlayingItem.MediaStreams | array | MediaStreams | MediaStream[] | Same in the spec. Filled in during playback in 2018. Not verified for current Emby. |
| MediaStream.Codec | string | Codec | string | Same |
| MediaStream.BitRate | int64 | BitRate | int32 | Same (compatible) |
| MediaStream.Channels | int | Channels | int32 | Same |
| MediaStream.Index | int | Index | int32 | Same |
| MediaStream.Width | int | Width | int32 | Same |
| MediaStream.Height | int | Height | int32 | Same |
| Session.TranscodingInfo | object | TranscodingInfo | TranscodingInfo | Same |
| TranscodingInfo.AudioCodec | string | AudioCodec | string | Same |
| TranscodingInfo.VideoCodec | string | VideoCodec | string | Same |
| TranscodingInfo.Container | string | Container | string | Same |
| TranscodingInfo.IsVideoDirect | bool | IsVideoDirect | boolean | Same |
| TranscodingInfo.IsAudioDirect | bool | IsAudioDirect | boolean | Same |
| TranscodingInfo.Bitrate | int64 | Bitrate | int32 | Same (compatible) |
| TranscodingInfo.CompletionPercentage | float64 | CompletionPercentage | double | Same |
| TranscodingInfo.Width | int | Width | int32 | Same |
| TranscodingInfo.Height | int | Height | int32 | Same |
| TranscodingInfo.AudioChannels | int | AudioChannels | int32 | Same |
| Not used | None | TranscodingInfo.TranscodeReasons | TranscodeReason[] (enum strings) | Present. dashbrr does not read it. |
| Query `ActiveWithinSeconds` | None | None | None | Missing from the spec |

## Sources

Emby primary sources:

- Emby REST API overview: https://dev.emby.media/doc/restapi/index.html
- API key authentication: https://dev.emby.media/doc/restapi/API-Key-Authentication.html
- User authentication: https://dev.emby.media/doc/restapi/User-Authentication.html
- `GET /System/Info`: https://dev.emby.media/reference/RestAPI/SystemService/getSystemInfo.html
- `GET /System/Info/Public`: https://dev.emby.media/reference/RestAPI/SystemService/getSystemInfoPublic.html
- `GET /Sessions`: https://dev.emby.media/reference/RestAPI/SessionsService/getSessions.html
- Emby SDK Swagger 2.0 spec: https://github.com/MediaBrowser/Emby.SDK/blob/1faf8176df7ede9152c38f27a5925e6ccaf5e12b/Documentation/Download/openapi_v2_noversion.json. The file has trailing commas, so strict JSON parsers reject it. This report removed the commas before it queried the file.
- Emby C# API key authenticator: https://github.com/MediaBrowser/Emby.ApiClients/blob/3f25bdbd17f017c9189650adca4af7bb42816459/Clients/Emby.ApiClient/Emby.ApiClient/Client/Authentication/EmbyApiKeyAuthenticator.cs
- Emby Go client configuration: https://github.com/MediaBrowser/Emby.ApiClients/blob/3f25bdbd17f017c9189650adca4af7bb42816459/Clients/Go/configuration.go
- Emby connectivity and ports: https://emby.media/support/articles/Connectivity.html

Old Emby server source (2018, the last open-source version, not current):

- https://github.com/MediaBrowser/Emby/blob/1d7c2ab4bfebdae31f89fdaabd3b68782ccf495a/Emby.Server.Implementations/HttpServer/Security/AuthorizationContext.cs
- https://github.com/MediaBrowser/Emby/blob/1d7c2ab4bfebdae31f89fdaabd3b68782ccf495a/Emby.Server.Implementations/HttpServer/HttpListenerHost.cs
- https://github.com/MediaBrowser/Emby/blob/1d7c2ab4bfebdae31f89fdaabd3b68782ccf495a/MediaBrowser.Api/System/SystemService.cs
- https://github.com/MediaBrowser/Emby/blob/1d7c2ab4bfebdae31f89fdaabd3b68782ccf495a/Emby.Server.Implementations/Session/SessionManager.cs

Jellyfin (tag `v12.2`, commit `ca0f16eb7c`):

- https://github.com/jellyfin/jellyfin/blob/v12.2/MediaBrowser.Model/System/PublicSystemInfo.cs
- https://github.com/jellyfin/jellyfin/blob/v12.2/MediaBrowser.Model/Dto/SessionInfoDto.cs
- https://github.com/jellyfin/jellyfin/blob/v12.2/Jellyfin.Api/Controllers/SessionController.cs
- Legacy auth disabled by default: https://github.com/jellyfin/jellyfin/pull/15559

This report used no secondary sources such as forum posts or blog posts.
