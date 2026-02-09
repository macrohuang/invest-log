# Invest Log

Invest Log is a local-first investment portfolio tracker built with a Go API
server and a single-page web app (SPA). It stores data in a local SQLite
database and can be packaged for desktop or mobile wrappers.

## Features
- Track accounts, holdings, and transactions
- Update and manage security prices (multi-source fallback)
- Portfolio allocation settings and history
- AI holdings analysis and suggestions (OpenAI-compatible API)
- Operation logs for audit and troubleshooting

## Project Layout
- `go-backend/`: Go API server (`cmd/server/main.go`)
- `static/`: SPA assets served by the Go server
- `ios/`: Capacitor iOS wrapper assets (public web bundle)
- `macos/`: macOS wrapper build helper

## Run Locally (Web/Desktop)
```bash
cd go-backend
go run ./cmd/server --data-dir /path/to/data --port 8000
```
Notes:
- The server auto-detects `../static` and serves it at `/`.
- Use `--web-dir` to point at a custom static directory.
- Environment variables: `INVEST_LOG_DATA_DIR` or `INVEST_LOG_DB_PATH`.

## Usage
- Open `http://127.0.0.1:8000` in a browser.
- If you open `static/index.html` directly (file/Capacitor), set the API base
  in Settings or pass `?api=http://127.0.0.1:8000`.
- Create accounts and asset types, add transactions, update prices, and review
  holdings and portfolio history.
- In Holdings view, use `AI Analyze` to generate AI-based portfolio insights and
  symbol-level suggestions.

## Packaging & Installation

### Web / PWA
Build is not required: the Go server serves the SPA from `static/`.
- Install: use the browser's "Install app" action when opened at
  `http://127.0.0.1:8000`.
- Use: launch the installed PWA, then configure the API base if needed.

### macOS (Wrapper + Backend)
Prerequisites: Go and Xcode Command Line Tools.

Build a DMG:
```bash
macos/build_dmg.sh
```
Install:
- Open the DMG at `output/macos/InvestLog-macOS-arm64.dmg`.
- Drag `InvestLog.app` into Applications.
Use:
- Launch the app; it starts the bundled backend and opens the SPA.

### iOS / iPadOS (Capacitor)
Prerequisites: Node.js + npm, Xcode.

Install dependencies (once):
```bash
npm install
```

Sync web assets into iOS:
```bash
scripts/sync_spa.sh
# or
npx cap sync ios
```

Open and build:
```bash
npx cap open ios
```
Notes:
- The iOS project is set to iPad-only (`TARGETED_DEVICE_FAMILY = 2`).
- Use Xcode to Run on an iPad simulator or device, or Archive for distribution.
- If `xcodebuild` complains about missing plugins, run
  `xcodebuild -runFirstLaunch` first.

CLI build (iPad simulator):
```bash
xcodebuild -project ios/App/App.xcodeproj \
  -scheme App \
  -configuration Release \
  -sdk iphonesimulator \
  -destination 'generic/platform=iOS Simulator' \
  -derivedDataPath output/ios-ipad-sim \
  build
```

Alternative native path:
- The Go backend exposes a gomobile wrapper in `go-backend/pkg/mobile`.
  See `go-backend/README.md` for building an XCFramework for a native iPad app.

## Git Large File Guard

To prevent large build artifacts from entering Git history again, this repo
includes a local `pre-commit` hook.

Install once per clone:
```bash
scripts/install_git_hooks.sh
```

What it blocks by default:
- Staged files larger than `20MB`
- Staged paths under build/output folders such as:
  `node_modules/`, `output/`, `src-tauri/target/`, `src-tauri/binaries/`,
  `build/`, `dist/`, `backend-dist/`, `logs/`, `__pycache__/`, `.qoder/repowiki/`

Optional:
- Change max file size for current shell:
  ```bash
  MAX_FILE_SIZE_MB=10 git commit -m "your message"
  ```
