# ankerctl-ng

Web UI and CLI for AnkerMake M5 and M5C printers on the local network. It talks
to the printer over Anker's PPPP and MQTT protocols and accepts uploads from
OrcaSlicer as an OctoPrint-compatible host.

[![Release](https://img.shields.io/github/v/release/FabianRingel/ankerctl-ng?include_prereleases)](https://github.com/FabianRingel/ankerctl-ng/releases)
[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![CI](https://github.com/FabianRingel/ankerctl-ng/actions/workflows/ci.yml/badge.svg)](https://github.com/FabianRingel/ankerctl-ng/actions/workflows/ci.yml)
[![Docker](https://img.shields.io/badge/ghcr.io-ankerctl--ng-blue?logo=docker)](https://ghcr.io/fabianringel/ankerctl-ng)

![ankerctl-ng dashboard](docs/img/screenshot-home-ng.png)

## Status

This repository continues [`jr551/ankerctl-ng`](https://github.com/jr551/ankerctl-ng),
which was archived in October 2026. Its author moved on to
[`ankerctlng-mcp`](https://github.com/jr551/ankerctlng-mcp), an MCP server that
still uses this project's HTTP bridge on port 4470 for G-code upload and camera
capture.

Work here so far is security hardening: the server refuses to listen on the
network without an API key, rejects cross-origin browser requests, no longer
returns stored credentials, and builds with a supported Go toolchain. See
[Security](#security).

The project is experimental. The upstream author developed it against an M5C.

## Features

- **Dashboard and control:** printer state, temperatures, Z offset, bed leveling,
  raw G-code, and an emergency stop that cuts power through a smart socket.
- **Uploads:** OctoPrint-compatible endpoint for OrcaSlicer. Uploads over PPPP
  use keepalives and health checks, and can power-cycle the printer through the
  smart socket when a transfer gets stuck.
- **Slicing in the browser:** STL and OpenSCAD via
  [polyslice](https://github.com/jgphilpott/polyslice), with an optional
  AI-assisted OpenSCAD editor.
- **Camera:** printer camera (M5), external RTSP/HTTP cameras, or a Home
  Assistant camera entity. The M5C has no built-in camera.
- **AI print monitoring (optional):** periodic camera checks with a vision
  model, evidence images in the print history, and an optional power cut when
  a failure or an animal is detected.
- **Filament library**, including colour detection from the camera.
- **Home Assistant:** MQTT discovery, camera entities, smart-socket control.
- **Notifications:** Apprise, SMTP, webhooks, Home Assistant TTS announcements.
- **Timelapse** recording and power-saving controls.

| | |
|---|---|
| Print history with AI evidence frames<br>![History with AI evidence](docs/img/screenshot-history-ai.png) | Slicing STL or OpenSCAD in the browser<br>![Slice & Build](docs/img/screenshot-slice-build.png) |
| AI-assisted OpenSCAD editing<br>![OpenSCAD AI result](docs/img/screenshot-ai-openscad-demo.png) | AI print monitoring<br>![AI monitor](docs/img/screenshot-ai-monitor.png) |

The AI features call an OpenAI-compatible chat-completions API (default: the
Kilo gateway at `api.kilo.ai`) and send camera frames, G-code previews or
OpenSCAD source to it. They are off until you configure an API key.

## Installation

### Docker

```sh
docker run -d \
  --name ankerctl-ng \
  --network host \
  -e ANKERCTL_API_KEY=<random, at least 16 chars of [A-Za-z0-9_-]> \
  -v ~/.ankerctl-ng:/home/ankerctl/.ankerctl-ng \
  -v ankerctl-ng-captures:/captures \
  ghcr.io/fabianringel/ankerctl-ng:latest
```

The container listens on all interfaces and will not start without
`ANKERCTL_API_KEY`. Host networking is required: PPPP uses UDP broadcasts and
asymmetric ports that NAT breaks.

With Docker Compose, copy `.env.example` to `.env`, set `ANKERCTL_API_KEY`, and
use the [`docker-compose.yml`](docker-compose.yml) in this repository:

```sh
docker compose up -d
```

To build the image yourself:

```sh
docker build -t ankerctl-ng .
```

### From source (Linux)

```sh
git clone https://github.com/FabianRingel/ankerctl-ng.git
cd ankerctl-ng
./install.sh install
```

`install.sh` installs Go if needed, builds and installs the binary, and
optionally sets up a systemd service listening on `0.0.0.0:4470`. If no API key
exists for the service user, it creates one and prints it once. Update with
`git pull && ./install.sh update`.

To build by hand (Go 1.27 or newer):

```sh
bash scripts/prepare-web-vendor.sh   # fetch vendored web assets
go build -o ankerctl-ng ./cmd/ankerctl/
go test ./...
```

## First steps

1. Open `http://YOUR-HOST:4470/?apikey=<key>` once. This sets a session cookie
   and removes the key from the URL.
2. Log in with your Anker account or import the login from the AnkerMake /
   eufyMake slicer. This fetches the printer's keys from the Anker cloud. File
   uploads and video then go directly to the printer over PPPP on the LAN;
   status and commands still use Anker's cloud MQTT broker.
3. Point OrcaSlicer at the server (see below).

## OrcaSlicer

- Host type: `OctoPrint`
- Host / URL: `http://YOUR-HOST:4470`
- API key: your `ANKERCTL_API_KEY`

Use "Send and Print" to start the job immediately.

## Security

- **API key:** required whenever the server listens on anything other than
  loopback. Generate one with `ankerctl-ng config api-key create` or any password
  generator, and pass it via `ANKERCTL_API_KEY`. Only if another layer already
  restricts access (firewall, authenticating reverse proxy) should you set
  `ANKERCTL_ALLOW_INSECURE=true`.
- **Cross-origin requests:** state-changing browser requests and WebSocket
  connections are only accepted from the UI's own origin. Add other origins
  with `ANKERCTL_TRUSTED_ORIGINS`. Behind a reverse proxy, set
  `ANKERCTL_TRUST_PROXY=true`.
- **Stored credentials** (Home Assistant tokens, MQTT password, AI and Apprise
  keys) are write-only: the API reports whether one is saved, never its value.
- **Network:** TCP 4470 for the UI; UDP 32100, 32108 and 32109 for PPPP. With a
  stateful firewall, allow those UDP ports inbound, see
  [`docs/operations/firewall.md`](docs/operations/firewall.md).
- Report vulnerabilities as described in [`SECURITY.md`](SECURITY.md).

## Configuration

All settings can be passed as environment variables; [`.env.example`](.env.example)
lists them with explanations. The config directory is `~/.ankerctl-ng`; an
existing `~/.ankerctl` is picked up automatically. Older migration and protocol
notes are in [`docs/`](docs/).

## Credits

This project builds on:

1. [`Ankermgmt/ankerctl`](https://github.com/Ankermgmt/ankerctl): the original
   Python tool and the `libflagship` library that reverse-engineered the
   AnkerMake MQTT, PPPP and HTTP/cloud protocols.
2. [`Django1982/ankerctl_go_remake`](https://github.com/Django1982/ankerctl_go_remake):
   the Go port of the Python original (see [`docs/MIGRATION_PLAN.md`](docs/MIGRATION_PLAN.md)).
   The Go module path `github.com/django1982/ankerctl` is kept from there.
3. [`jr551/ankerctl-ng`](https://github.com/jr551/ankerctl-ng): the
   `ankerctl-ng` fork that added AI print monitoring, PPPP upload recovery,
   smart-socket and Home Assistant integration, notifications and the current UI.

[`NOTICE`](NOTICE) records the full attribution chain.
[`CONTRIBUTING.md`](CONTRIBUTING.md) explains how to contribute.

## License

GNU General Public License v3.0, like all upstream projects. See [`LICENSE`](LICENSE).
