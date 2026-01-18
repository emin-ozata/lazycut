package video

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type TrimState struct {
	InPoint  *time.Duration
	OutPoint *time.Duration
}

func (t *TrimState) SetIn(pos time.Duration) {
	if t.OutPoint != nil && pos > *t.OutPoint {
		t.OutPoint = nil
	}
	t.InPoint = &pos
}

func (t *TrimState) SetOut(pos time.Duration) {
	if t.InPoint != nil && pos < *t.InPoint {
		t.InPoint = nil
	}
	t.OutPoint = &pos
}

func (t *TrimState) Clear() {
	t.InPoint = nil
	t.OutPoint = nil
}

func (t *TrimState) IsComplete() bool {
	return t.InPoint != nil && t.OutPoint != nil
}

func (t *TrimState) Duration() time.Duration {
	if !t.IsComplete() {
		return 0
	}
	return *t.OutPoint - *t.InPoint
}

type Player struct {
	path       string
	duration   time.Duration
	position   time.Duration
	playing    bool
	fps        int
	width      int
	height     int
	properties *VideoProperties
	quality    QualityPreset

	mu           sync.Mutex
	currentFrame string
	stopChan     chan struct{}

	// Optimization: Frame cache
	cache *FrameCache

	Trim TrimState
}

func NewPlayer(path string) (*Player, error) {
	props, err := GetVideoProperties(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	return &Player{
		path:       path,
		duration:   props.Duration,
		position:   0,
		playing:    false,
		fps:        int(props.FPS),
		properties: props,
		quality:    QualityMedium,
		stopChan:   make(chan struct{}),
		cache:      NewFrameCache(DefaultCacheCapacity, props.FPS),
	}, nil
}

func (p *Player) SetSize(width, height int) {
	p.mu.Lock()
	oldWidth, oldHeight := p.width, p.height
	p.width = width
	p.height = height
	pos := p.position
	quality := p.quality
	playing := p.playing
	p.mu.Unlock()

	if !playing && width > 0 && height > 0 && (width != oldWidth || height != oldHeight) {
		p.renderFrameCached(pos, width, height, quality)
	}
}

func (p *Player) Play() error {
	p.mu.Lock()
	if p.playing {
		p.mu.Unlock()
		return nil
	}
	p.playing = true
	p.stopChan = make(chan struct{})
	p.mu.Unlock()

	go p.playbackLoop()
	return nil
}

func (p *Player) Pause() {
	p.mu.Lock()
	if !p.playing {
		p.mu.Unlock()
		return
	}
	p.playing = false
	close(p.stopChan)
	pos := p.position
	width, height := p.width, p.height
	quality := p.quality
	p.mu.Unlock()

	if width > 0 && height > 0 {
		p.renderFrameCached(pos, width, height, quality)
	}
}

func (p *Player) Toggle() error {
	p.mu.Lock()
	playing := p.playing
	p.mu.Unlock()

	if playing {
		p.Pause()
		return nil
	}
	return p.Play()
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.position
}

func (p *Player) Seek(position time.Duration) {
	p.mu.Lock()
	if position < 0 {
		position = 0
	}
	if position > p.duration {
		position = p.duration
	}
	p.position = position
	width, height := p.width, p.height
	quality := p.quality
	playing := p.playing
	p.mu.Unlock()

	if !playing && width > 0 && height > 0 {
		p.renderFrameCached(position, width, height, quality)
	}
}

func (p *Player) FPS() int {
	return p.fps
}

func (p *Player) Path() string {
	return p.path
}

func (p *Player) Duration() time.Duration {
	return p.duration
}

func (p *Player) Properties() *VideoProperties {
	return p.properties
}

func (p *Player) CurrentFrame() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentFrame
}

func (p *Player) Quality() QualityPreset {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.quality
}

func (p *Player) CycleQuality() QualityPreset {
	p.mu.Lock()
	p.quality = p.quality.Next()
	newQuality := p.quality
	pos := p.position
	width, height := p.width, p.height
	playing := p.playing
	p.mu.Unlock()

	if !playing && width > 0 && height > 0 {
		p.renderFrameCached(pos, width, height, newQuality)
	}
	return newQuality
}

func (p *Player) Close() {
	p.Pause()
}

func (p *Player) playbackLoop() {
	frameInterval := time.Second / time.Duration(p.fps)
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.mu.Lock()
			pos := p.position
			width := p.width
			height := p.height
			quality := p.quality
			p.mu.Unlock()

			if width <= 0 || height <= 0 {
				continue
			}

			// Try cache first for playback
			if frame, ok := p.cache.Get(pos, width, height, quality); ok {
				p.mu.Lock()
				p.currentFrame = frame
				p.position += frameInterval
				if p.position >= p.duration {
					p.position = p.duration
					p.playing = false
					p.mu.Unlock()
					return
				}
				p.mu.Unlock()
				continue
			}

			// Cache miss - render synchronously for playback
			frame, err := p.renderFrame(pos, width, height)
			if err == nil {
				p.cache.Put(pos, width, height, quality, frame)
				p.mu.Lock()
				p.currentFrame = frame
				p.position += frameInterval
				if p.position >= p.duration {
					p.position = p.duration
					p.playing = false
					p.mu.Unlock()
					return
				}
				p.mu.Unlock()
			}
		}
	}
}

// renderFrameCached renders a frame using cache
func (p *Player) renderFrameCached(position time.Duration, width, height int, quality QualityPreset) {
	// Check cache first
	if frame, ok := p.cache.Get(position, width, height, quality); ok {
		p.mu.Lock()
		p.currentFrame = frame
		p.mu.Unlock()
		return
	}

	// Cache miss - render
	frame, err := p.renderFrame(position, width, height)
	if err != nil {
		return
	}
	p.cache.Put(position, width, height, quality, frame)
	p.mu.Lock()
	p.currentFrame = frame
	p.mu.Unlock()
}

func (p *Player) renderFrame(position time.Duration, width, height int) (string, error) {
	p.mu.Lock()
	config := ChafaPresets[p.quality]
	p.mu.Unlock()

	ffmpegCmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", position.Seconds()),
		"-i", p.path,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "bmp",
		"-loglevel", "error",
		"-",
	)

	chafaArgs := config.BuildArgs(width, height)
	chafaCmd := exec.Command("chafa", chafaArgs...)

	pipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	chafaCmd.Stdin = pipe

	var chafaOut bytes.Buffer
	chafaCmd.Stdout = &chafaOut

	if err := chafaCmd.Start(); err != nil {
		return "", err
	}
	if err := ffmpegCmd.Run(); err != nil {
		return "", err
	}
	if err := chafaCmd.Wait(); err != nil {
		return "", err
	}

	return chafaOut.String(), nil
}

func CheckDependencies() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found. Install: brew install ffmpeg")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found. Install: brew install ffmpeg")
	}
	if _, err := exec.LookPath("chafa"); err != nil {
		return fmt.Errorf("chafa not found. Install: brew install chafa")
	}
	return nil
}
