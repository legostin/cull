# cull

Interactive TUI disk space analyzer. Scan directories, find what's eating your disk, and delete it — all from the terminal.

```
   ___ _   _ | | |
  / __| | | || | |      154.2 GB free
 | (__| |_| || | |
  \___|\__,_||_|_|

 /Users/you/Downloads
────────────────────────────────────
      SIZE  NAME
   12.4 GB  old-project/
    3.2 GB  video.mp4
    1.1 GB  archive.zip
     845 MB  node_modules/
```

## Install

```
brew tap legostin/tap
brew install cull
```

Or build from source:

```
go install github.com/legostin/cull@latest
```

## Usage

```
cull              # scan current directory
cull ~/Downloads  # scan specific path
```

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` or arrow keys | Navigate up/down |
| `g` / `G` | Jump to top/bottom |
| `enter` | Enter directory |
| `backspace` / `esc` | Go to parent directory |
| `s` | Toggle selection |
| `S` | Range select (from last selection to cursor) |
| `d` | Delete selected (or item under cursor) |
| `f` | Filter by name |
| `space` | Quick Look preview (macOS) |
| `q` / `ctrl+c` | Quit |

## How it works

1. **Quick scan** — instantly lists files and directories with file sizes
2. **Background sizing** — recursively computes directory sizes while you browse
3. **Caching** — previously visited directories load instantly
4. **Live sorting** — entries re-sort by size as directory sizes are computed

## License

MIT
