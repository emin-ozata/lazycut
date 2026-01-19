package video

import (
	"fmt"
	"os/exec"
	"sync"
)

// AudioPlayer manages audio playback via ffplay subprocess
type AudioPlayer struct {
	filePath string
	cmd      *exec.Cmd
	muted    bool
	mu       sync.Mutex
}

// NewAudioPlayer creates a new AudioPlayer for the given video file
func NewAudioPlayer(filePath string) *AudioPlayer {
	return &AudioPlayer{
		filePath: filePath,
		muted:    false,
	}
}

// Start spawns ffplay to play audio from the given position
func (a *AudioPlayer) Start(position float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.muted {
		return
	}

	// Stop any existing playback
	a.stopLocked()

	a.cmd = exec.Command("ffplay",
		"-nodisp",
		"-autoexit",
		"-vn",
		"-ss", formatSeconds(position),
		"-loglevel", "quiet",
		a.filePath,
	)

	// Start ffplay in background
	_ = a.cmd.Start()
}

// Stop kills the ffplay process if running
func (a *AudioPlayer) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
}

// stopLocked stops playback (must be called with lock held)
func (a *AudioPlayer) stopLocked() {
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
		a.cmd = nil
	}
}

// ToggleMute toggles the muted state and stops audio if muting
func (a *AudioPlayer) ToggleMute() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.muted = !a.muted
	if a.muted {
		a.stopLocked()
	}
}

// IsRunning returns true if ffplay is currently active
func (a *AudioPlayer) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd == nil || a.cmd.Process == nil {
		return false
	}
	// Check if process is still running by checking if Wait would block
	// A nil from ProcessState means it's still running
	return a.cmd.ProcessState == nil
}

// IsMuted returns the current mute state
func (a *AudioPlayer) IsMuted() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.muted
}

// formatSeconds formats a float64 seconds value for ffplay -ss argument
func formatSeconds(seconds float64) string {
	return fmt.Sprintf("%.3f", seconds)
}
