# self-checkout-pos

Go HTTP service that runs as a Windows service and self-updates. Two binaries on the host: `server.exe` polls GitHub Releases, `updater.exe` handles the file swap and service restart.

## Architecture

```mermaid
flowchart LR
    subgraph host["Customer Windows host (C:\self-checkout-pos)"]
        direction TB
        svc["server.exe<br/>HTTP :7000<br/>SCM-managed"]
        upd["updater.exe<br/>spawned on apply"]
        stage[(".update/&lt;version&gt;/<br/>staged downloads")]
        live[("server.exe.old<br/>updater.exe.old<br/>status.json")]
        svc -- "hourly poll + verify" --> stage
        svc -- "POST /update<br/>or 02:00-05:00 window" --> upd
        upd -- "sc stop / sc start" --> svc
        upd -- "rename + copy" --> live
        upd -- "writes" --> live
    end
    subgraph gh["GitHub Releases"]
        direction TB
        man["manifest.json"]
        bin["server.exe + updater.exe<br/>+ .sha256 sidecars"]
    end
    svc -. "GET manifest.json" .-> man
    svc -. "GET assets + verify sha256" .-> bin
```

Windows won't let a service overwrite its own running binary. `server.exe` handles poll, download, checksum, and handoff. `updater.exe` runs detached for SCM stop, rename, copy, restart, and health check. Failed health check triggers rollback from `.old` files.

## Layout

```
.
├── cmd/
│   ├── server/main.go        # service entry point
│   └── updater/main.go       # update helper entry point
├── internal/
│   ├── applier/              # stop, swap, start, poll, rollback
│   ├── config/               # config.json loader
│   ├── server/               # HTTP server, middleware, handlers
│   ├── service/              # kardianos/service wrapper
│   └── updater/              # manifest fetch, download staging, handoff
└── .github/workflows/
    └── release.yml           # semantic-release + asset upload
```

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o server.exe  ./cmd/server
GOOS=windows GOARCH=amd64 go build -o updater.exe ./cmd/updater

# Local check
go build ./...
```

CI bakes the version: `-ldflags "-X main.version=v0.1.0"`. Without it, binary identifies as `dev` and updater stays silent.

## Run locally (macOS / Linux)

```bash
cp config.example.json config.json
POS_CONFIG=$(pwd)/config.json go run ./cmd/server -action run
curl localhost:7000/health
```

Service wrapper drops to foreground outside Windows. `Ctrl+C` stops it.

## Install on Windows

1. Build both binaries with a version stamp.
2. Drop `server.exe`, `updater.exe`, and `config.json` into `C:\self-checkout-pos\`.
3. Register and start:
   ```cmd
   server.exe -action install
   server.exe -action start
   ```

Other actions: `stop`, `uninstall`, `run` (foreground).

## Configuration

`config.json` lives beside the binary. On dev machines, set `POS_CONFIG`.

```json
{
  "port": 7000,
  "api_key": "change-me",
  "auto_update_enabled": true
}
```

| Field                 | Default     | Notes                                                                        |
| --------------------- | ----------- | ---------------------------------------------------------------------------- |
| `port`                | `7000`      | HTTP listen port.                                                            |
| `api_key`             | `change-me` | Required in `X-API-Key` header (or `?api_key=` query) on auth-gated routes. |
| `auto_update_enabled` | `true`      | Kill switch for the updater poll loop.                                       |

## HTTP endpoints

| Method | Path      | Auth    | Description                                                                                                                               |
| ------ | --------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`  | `/health` | none    | `{ version, auto_update_enabled, started_at, uptime, last_poll_at, last_update }`. Updater uses this to confirm new binary booted. |
| `GET`  | `/hello`  | API key | Returns `{ message, version }`.                                                                                                           |
| `POST` | `/update` | API key | Forces immediate poll and apply, ignoring nightly window.                                                                                 |
| `POST` | `/config` | API key | Replaces `config.json` and reloads. Port changes need a service restart.                                                                  |

## Update flow

```mermaid
sequenceDiagram
    participant S as server.exe
    participant GH as GitHub Releases
    participant U as updater.exe
    participant FS as Disk

    loop every hour
        S->>GH: GET manifest.json
        GH-->>S: { version, urls, sha256 urls }
        alt newer version
            S->>GH: GET server.exe / updater.exe + .sha256
            S->>FS: write to .update/{version}/, verify checksums
        end
    end
    Note over S: wait for 02:00-05:00 window<br/>(or POST /update)
    S->>U: spawn detached with --new-exe, --new-updater
    U->>S: sc.exe stop
    U->>FS: rename live -> .old, copy staged -> live
    U->>S: sc.exe start
    U->>S: GET /health (up to 60s)
    alt health reports new version
        U->>FS: delete .old, write status.json: ok
    else timeout or wrong version
        U->>FS: restore .old, mark binary as .failed-{version}
        U->>S: sc.exe start (rollback)
        U->>FS: write status.json: rolled_back
    end
```

Notes:
- `dev` builds never poll.
- First boot on an older `server.exe` bootstraps a matching `updater.exe` from the release asset.
- Stage dirs for applied versions are pruned on next boot.

## Pointing the updater at a different repo

Default manifest URL:
```
https://github.com/tqrcisio/self-checkout-pos/releases/latest/download/manifest.json
```

Override at build time:
```bash
go build -ldflags \
  "-X main.version=v0.1.0 \
   -X github.com/tqrcisio/self-checkout-pos/internal/updater.DefaultReleaseRepo=youruser/yourrepo" \
  -o server.exe ./cmd/server
```

Or at runtime:
```bash
POS_RELEASE_REPO=youruser/yourrepo server.exe -action run
POS_MANIFEST_URL=https://example.com/manifest.json server.exe -action run
```

`manifest.json` shape:
```json
{
  "version":              "v0.1.0",
  "released_at":          "2026-05-18T00:00:00Z",
  "download_url":         "https://github.com/owner/repo/releases/download/v0.1.0/server.exe",
  "sha256_url":           "https://github.com/owner/repo/releases/download/v0.1.0/server.exe.sha256",
  "updater_download_url": "https://github.com/owner/repo/releases/download/v0.1.0/updater.exe",
  "updater_sha256_url":   "https://github.com/owner/repo/releases/download/v0.1.0/updater.exe.sha256"
}
```

## Release pipeline

```mermaid
flowchart LR
    push["push to main<br/>(conventional commit)"]
    semrel["go-semantic-release<br/>tag + release notes"]
    build["go build -ldflags<br/>server.exe + updater.exe"]
    sums["sha256 sidecars"]
    manifest["manifest.json"]
    upload["gh release upload"]
    poll[("server.exe on host<br/>picks it up next hour")]

    push --> semrel --> build --> sums --> manifest --> upload --> poll
```

Commit conventions:
```
feat:     minor bump
fix:      patch bump
feat!:    major bump
chore / docs / refactor:  no release
```

## Dependencies

- [`github.com/kardianos/service`](https://github.com/kardianos/service) - cross-platform service wrapper
- [`github.com/kardianos/osext`](https://github.com/kardianos/osext) - resolves executable directory

Both CGO-free.

## License

MIT. See [LICENSE](./LICENSE).
