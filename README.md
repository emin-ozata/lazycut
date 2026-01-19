# lazycut

Terminal-based video trimming tool. Mark in/out points and export trimmed clips with aspect ratio control.

![lazycut demo](media/demo.gif)

## Install

```bash
brew tap emin-ozata/homebrew-tap
brew install lazycut
```

Or build from source:
```bash
git clone https://github.com/eminozata/lazycut.git
cd lazycut
go build
./lazycut video.mp4
```

## Usage

```
lazycut <video-file>
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `h` / `l` | Seek ±1s |
| `H` / `L` | Seek ±5s |
| `i` / `o` | Set in/out points |
| `Enter` | Export |
| `?` | Help |
| `q` | Quit |

Repeat counts work: `5l` = seek forward 5 seconds.
