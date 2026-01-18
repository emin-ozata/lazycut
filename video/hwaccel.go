package video

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

type HWAccelType string

const (
	HWAccelNone        HWAccelType = ""
	HWAccelVideoToolbox HWAccelType = "videotoolbox" // macOS
	HWAccelVAAPI       HWAccelType = "vaapi"         // Linux
	HWAccelCUDA        HWAccelType = "cuda"          // NVIDIA
	HWAccelDXVA2       HWAccelType = "dxva2"         // Windows
)

type HWAccelConfig struct {
	Type      HWAccelType
	Available bool
	ErrorRate float64 // Track error rate for adaptive fallback
}

var (
	hwAccelOnce   sync.Once
	hwAccelConfig HWAccelConfig
)

// DetectHWAccel detects available hardware acceleration
// Called once at startup via sync.Once
func DetectHWAccel() HWAccelConfig {
	hwAccelOnce.Do(func() {
		hwAccelConfig = detectHWAccelImpl()
	})
	return hwAccelConfig
}

func detectHWAccelImpl() HWAccelConfig {
	availableAccels := getAvailableHWAccels()

	switch runtime.GOOS {
	case "darwin":
		if contains(availableAccels, "videotoolbox") {
			return HWAccelConfig{Type: HWAccelVideoToolbox, Available: true}
		}
	case "linux":
		// Prefer CUDA if available (usually faster)
		if contains(availableAccels, "cuda") {
			return HWAccelConfig{Type: HWAccelCUDA, Available: true}
		}
		if contains(availableAccels, "vaapi") {
			return HWAccelConfig{Type: HWAccelVAAPI, Available: true}
		}
	case "windows":
		if contains(availableAccels, "dxva2") {
			return HWAccelConfig{Type: HWAccelDXVA2, Available: true}
		}
		if contains(availableAccels, "cuda") {
			return HWAccelConfig{Type: HWAccelCUDA, Available: true}
		}
	}

	return HWAccelConfig{Type: HWAccelNone, Available: false}
}

func getAvailableHWAccels() []string {
	cmd := exec.Command("ffmpeg", "-hwaccels")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var accels []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "Hardware acceleration methods:" {
			accels = append(accels, line)
		}
	}
	return accels
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// BuildFFmpegArgs builds FFmpeg arguments with optional hardware acceleration
func BuildFFmpegArgs(path string, position float64, useHWAccel bool, hwConfig HWAccelConfig) []string {
	args := []string{}

	// Hardware acceleration must come before -i
	if useHWAccel && hwConfig.Available && hwConfig.Type != HWAccelNone {
		args = append(args, "-hwaccel", string(hwConfig.Type))
	}

	args = append(args,
		"-ss", formatSeconds(position),
		"-i", path,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "bmp",
		"-loglevel", "error",
		"-",
	)

	return args
}

func formatSeconds(seconds float64) string {
	return strings.TrimRight(strings.TrimRight(
		fmt.Sprintf("%.3f", seconds), "0"), ".")
}

// GetHWAccelStatus returns a human-readable status of hardware acceleration
func GetHWAccelStatus() string {
	config := DetectHWAccel()
	if !config.Available {
		return "Software decoding"
	}
	switch config.Type {
	case HWAccelVideoToolbox:
		return "VideoToolbox (macOS)"
	case HWAccelVAAPI:
		return "VAAPI (Linux)"
	case HWAccelCUDA:
		return "CUDA (NVIDIA)"
	case HWAccelDXVA2:
		return "DXVA2 (Windows)"
	default:
		return "Software decoding"
	}
}
