More my utilities on site [legost.in](https://legost.in/en/utilities)
# cull
```
              | | |
   / __| | | || | | 
  | (__| |_| || | | 
   \___|\__,_||_|_|
```
Interactive TUI disk space analyzer. Scan directories, find what's eating your disk, and delete it — all from the terminal.
<img width="978" height="689" alt="image" src="https://github.com/user-attachments/assets/a118aa4e-e64c-4ab1-b080-a40e36a1332a" />

## Install

### macOS

```
brew tap legostin/tap
brew install cull
```

### Linux (apt)

```
echo "deb [trusted=yes] https://legostin.github.io/apt-repo/ /" | sudo tee /etc/apt/sources.list.d/cull.list
sudo apt update
sudo apt install cull
```

### From source

```
go install github.com/legostin/cull@latest
```

## Usage

```
cull                        # scan current directory
cull ~/Downloads            # scan specific path
cull --read-only            # browse without deletion
cull -y                     # skip delete confirmation
cull -n 5000                # show up to 5000 items in Largest tab
cull --no-mouse             # disable mouse support
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

### Clear app caches safely

The CACHES tab finds known caches — dev tools (npm, go, cargo, Xcode…), browsers, messengers (Telegram, WhatsApp, Slack, Discord) and popular apps — and lets you trash or delete them. Docker is cleaned via `docker system prune -a -f` with confirmation.

### Reclaim build artifacts from old projects

The PROJECTS tab scans down from the current directory for project build artifacts — `node_modules`, Rust `target/`, Python `.venv`, Gradle `build/`, Go/PHP `vendor/`, CocoaPods, Terraform and more. Each row shows the artifact size and how long the project has been untouched; projects idle for 6+ months are highlighted green as safe to clean. Artifacts are matched only next to their project marker file (`package.json`, `Cargo.toml`, `go.mod`, …), so an unrelated `build/` directory is never touched.

### See the map

Press `m` on the Browse tab to switch to a treemap: every file and directory becomes a rectangle sized by bytes. Move between rectangles with `h/j/k/l`, `enter` zooms into a directory, `esc` goes up — and `s`/`d` select and delete right from the map.

### Use the mouse

Click a row or a map rectangle to move the cursor, double-click to enter a directory, right-click to select, scroll with the wheel, click tab names to switch. Hold Shift (or Option on macOS) to select terminal text as usual; `--no-mouse` turns mouse support off entirely.

### Filter by name

Press `f` and type to instantly filter entries. Great for finding files by extension.

![filter](docs/filtration.gif)

### Honest sizes

Sizes are what you'd actually free by deleting — allocated blocks, not logical length (`du` semantics). Sparse disk images (Docker, OrbStack, UTM) no longer show terabytes that don't exist, hardlinked files are counted once, and on macOS APFS clones (Xcode simulators, Chrome's update clone) report only their private bytes. Network mounts and other filesystems are never crossed, so scanning your home dir won't wake a sleeping VM.

### System paths are protected

OS-managed locations — `/var/folders`, `/private/tmp`, `/System`, app Containers, `/proc` on Linux — carry a `⚠ sys` badge, and deleting anything there always asks for confirmation, even with `-y`: those files belong to running apps and clean themselves up.

### Fast

The deep scan uses a work-stealing parallel walk across all cores, and on macOS reads directory attributes in bulk (`getattrlistbulk`) — one syscall covers dozens of files.

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
| `m` | Toggle treemap view (Browse) |
| `h` | Toggle hidden files |
| `t` | Cycle sort mode (size / name / updated / created); size / idle on Projects |
| `r` | Rescan (Projects) / restore (History) |
| `tab` | Toggle trash / permanent delete |
| `shift+tab` | Switch tabs (Browse / Largest / Caches / Projects / History) |
| `space` | Quick Look preview (macOS) |
| `ctrl+l` | Force full repaint |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

## License

MIT
