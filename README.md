# cull

Interactive TUI disk space analyzer. Scan directories, find what's eating your disk, and delete it — all from the terminal.

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
cull                        # scan current directory
cull ~/Downloads            # scan specific path
cull ~/Downloads ~/Desktop  # scan multiple roots
```

## Features

### Browse and navigate

Instantly see what's taking up space. Directories are sized in the background while you browse — entries re-sort as sizes come in.

![browse](docs/selection.gif)

### Select and delete safely

Select files with `s`, range-select with `S`, then `d` to move to trash. Easy to undo.

![safe delete](docs/safe.gif)

### Permanent delete

Switch to permanent mode with `tab` when you're sure. Confirmation dialog keeps you safe.

![permanent delete](docs/permanent.gif)

### Find the largest files

`shift+tab` switches to the Largest tab — a deep scan across all subdirectories to surface the biggest files.

![largest files](docs/largest.gif)

### Filter by name

Press `f` and type to instantly filter entries. Great for finding files by extension.

![filter](docs/filtration.gif)

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` or arrow keys | Navigate up/down |
| `g` / `G` | Jump to top/bottom |
| `enter` | Enter directory |
| `backspace` / `esc` | Go to parent directory |
| `s` | Toggle selection |
| `S` | Range select |
| `d` | Delete selected |
| `e` | Dry-run preview |
| `f` | Filter by name |
| `h` | Toggle hidden files |
| `t` | Cycle sort mode (size / name / updated / created) |
| `tab` | Toggle trash / permanent delete |
| `shift+tab` | Switch between Browse and Largest tabs |
| `space` | Quick Look preview (macOS) |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

## License

MIT
