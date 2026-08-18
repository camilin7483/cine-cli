# 🎬 cine-cli

**Watch movies and TV shows from your terminal** — inspired by [ani-cli](https://github.com/pystardust/ani-cli).

```
cine                  # interactive TUI
cine search "inception"
cine play "breaking bad" --season 1 --episode 1
cine doctor
```

Developed by **CamiloDev** · version **1.0.0**

---

## Features

| Feature | Status |
|---------|--------|
| Interactive TUI (Bubble Tea) | ✅ |
| Search movies & series (TMDB) | ✅ |
| Season / episode picker | ✅ |
| Stream resolve → mpv/vlc | ✅ |
| Continue watching | ✅ |
| Favorites / history / bookmarks | ✅ |
| Offline play of downloads | ✅ |
| Subtitles (OpenSubtitles, optional key) | ✅ |
| Spanish / English UI | ✅ |
| Plugins | ✅ |
| Docker | ✅ |

## Requirements

- **Go 1.22+** (to build) or a release binary
- **mpv** or **vlc** (player)
- **Google Chrome / Chromium** (required to decrypt stream sources)
- **TMDB API key** (free): https://www.themoviedb.org/settings/api

Optional:
- `yt-dlp` — extra fallback
- OpenSubtitles API key — external subtitles

## Install

```bash
# from source
git clone <repo> && cd cine-cli
go build -o cine ./cmd/cine
sudo mv cine /usr/local/bin/

# config
mkdir -p ~/.config/cine-cli
cine config set tmdb_api_key YOUR_KEY
cine config set language es
cine doctor
```

## Usage

### TUI (default)

```bash
cine
```

Keys:

| Key | Action |
|-----|--------|
| type + enter | Search |
| ↑↓ / j k | Move |
| enter | Select / play |
| t | Trending |
| s | Sidebar |
| l | Cycle preferred audio language |
| b | Open current title in browser |
| f | Favorite |
| ? | Help |
| esc | Back |
| q / ctrl+c | Quit |

### CLI

```bash
cine search "dune"
cine play "dune"                 # movie
cine play "breaking bad" -s 1 -e 1
cine trending
cine history
cine favorites
cine offline list
cine offline play file.mp4
cine bookmarks list
cine config get tmdb_api_key
cine config set player mpv
cine providers
cine doctor
```

## How streams work

1. Metadata from **TMDB**
2. Source resolve via **vidsrc** data API (WASM decrypt in headless Chrome)
3. CDN JWT from `/generate.php` attached as `?token=`
4. Preflight HTTP check (no window flash on dead links)
5. Play in **mpv** with Referer / headers

**Not every title is available.** If the upstream catalog returns 404, cine-cli reports it cleanly — it will not open a browser unless you press `b`.

## Config

`~/.config/cine-cli/config.yaml`

```yaml
language: es
player: mpv
tmdb_api_key: "..."
subtitles_enabled: true
subtitles_language: es
opensubtitles_api_key: ""
theme_mode: dark
provider: vidsrc
```

## Docker

```bash
docker build -t cine-cli .
docker run --rm -it \
  -e TMDB_API_KEY=... \
  -v /path/to/config:/root/.config/cine-cli \
  cine-cli doctor
```

Note: playback needs host mpv and display; Docker is best for CLI search/doctor.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| TMDB search fails | `cine config set tmdb_api_key …` |
| Always “no stream” | Install Chromium/Chrome; check `cine doctor` |
| mpv 403/428 | Fixed in 1.0 via token + preflight; try another source |
| Title missing from catalog | Upstream gap — try another title or `b` for browser |
| Episode list cuts header | Fixed in 1.0 (single-line rows + pinned title) |

## Disclaimer

This tool is for educational purposes. It uses third-party embed sources that may change or geo-block. Respect copyright laws in your country.

## License

MIT
