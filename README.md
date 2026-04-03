<p align="center">
  <img src="assets/icon.png" width="128" alt="doq">
  <h1 align="center">doq</h1>
  <p align="center">Query Apple developer documentation from your terminal</p>
</p>

<p align="center">
  <a href="https://github.com/Aayush9029/doq/releases/latest"><img src="https://img.shields.io/github/v/release/Aayush9029/doq" alt="Release"></a>
  <a href="https://github.com/Aayush9029/doq/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Aayush9029/doq" alt="License"></a>
</p>

<p align="center">

https://github.com/user-attachments/assets/9d4c5154-8fe9-437f-9f03-b287cb7188af

</p>

## Install

```bash
brew install aayush9029/tap/doq
```

Or tap first:

```bash
brew tap aayush9029/tap
brew install doq
```

## Usage

```bash
doq                          # launch interactive TUI
doq search View              # search for symbols
doq info View                # full declaration + docs
doq list                     # list indexed frameworks
doq index                    # build index (curated ~30 frameworks)
doq index Swift Foundation   # index specific frameworks
doq index --all              # index all ~295 SDK frameworks
doq docs                     # launch semantic docs TUI (macOS 26+)
doq docs search "swift testing"
doq docs get /documentation/Testing
```

## How it works

1. Runs `xcrun swift symbolgraph-extract` to generate JSON symbol graphs from Xcode's SDKs
2. Parses symbol graphs for declarations, doc comments, availability, and relationships
3. Builds a SQLite FTS5 index at `~/.local/share/doq/index.db`
4. Queries the index with ranked full-text search

On macOS 26+, `doq docs` uses Apple's local documentation asset plus the system embedding and vector search frameworks.

## License

MIT
