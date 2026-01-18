package video

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AspectRatio int

const (
	AspectOriginal AspectRatio = iota
	Aspect16x9                 // Landscape
	Aspect9x16                 // Portrait/Mobile
	Aspect1x1                  // Square
	Aspect4x5                  // Instagram Portrait
)

var AspectRatioOptions = []struct {
	Ratio AspectRatio
	Label string
	W, H  int // ratio components (0,0 means original)
}{
	{AspectOriginal, "Original", 0, 0},
	{Aspect16x9, "16:9", 16, 9},
	{Aspect9x16, "9:16", 9, 16},
	{Aspect1x1, "1:1", 1, 1},
	{Aspect4x5, "4:5", 4, 5},
}

type ExportOptions struct {
	Input       string
	Output      string
	InPoint     time.Duration
	OutPoint    time.Duration
	AspectRatio AspectRatio
	Width       int
	Height      int
}

func BuildFFmpegCommand(opts ExportOptions) string {
	output := opts.Output
	if output == "" {
		output = generateOutputName(opts.Input)
	}
	duration := opts.OutPoint - opts.InPoint

	args := []string{"ffmpeg", "-y",
		"-ss", fmt.Sprintf("%.3f", opts.InPoint.Seconds()),
		"-i", filepath.Base(opts.Input),
		"-t", fmt.Sprintf("%.3f", duration.Seconds()),
	}

	if opts.AspectRatio != AspectOriginal && opts.Width > 0 && opts.Height > 0 {
		cropFilter := buildCropFilter(opts.Width, opts.Height, opts.AspectRatio)
		if cropFilter != "" {
			args = append(args, "-vf", cropFilter)
		}
	} else {
		args = append(args, "-c", "copy")
	}

	args = append(args, filepath.Base(output))
	return strings.Join(args, " ")
}

func ExportWithProgress(opts ExportOptions, progress chan<- float64) (string, error) {
	defer close(progress)

	output := opts.Output
	if output == "" {
		output = generateOutputName(opts.Input)
	} else {
		dir := filepath.Dir(opts.Input)
		ext := filepath.Ext(opts.Input)
		if filepath.Ext(output) == "" {
			output = output + ext
		}
		if !filepath.IsAbs(output) {
			output = filepath.Join(dir, output)
		}
	}
	duration := opts.OutPoint - opts.InPoint
	totalMicros := float64(duration.Microseconds())

	args := []string{"-y",
		"-ss", fmt.Sprintf("%.3f", opts.InPoint.Seconds()),
		"-i", opts.Input,
		"-t", fmt.Sprintf("%.3f", duration.Seconds()),
		"-progress", "pipe:2",
	}

	if opts.AspectRatio != AspectOriginal && opts.Width > 0 && opts.Height > 0 {
		cropFilter := buildCropFilter(opts.Width, opts.Height, opts.AspectRatio)
		if cropFilter != "" {
			args = append(args, "-vf", cropFilter)
		}
	} else {
		args = append(args, "-c", "copy")
	}

	args = append(args, output)

	cmd := exec.Command("ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_us=") {
			timeStr := strings.TrimPrefix(line, "out_time_us=")
			if micros, err := strconv.ParseFloat(timeStr, 64); err == nil && totalMicros > 0 {
				p := micros / totalMicros
				if p > 1.0 {
					p = 1.0
				}
				select {
				case progress <- p:
				default:
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w", err)
	}

	progress <- 1.0
	return output, nil
}

func Export(opts ExportOptions) (string, error) {
	progress := make(chan float64, 10)
	go func() {
		for range progress {
		}
	}()
	return ExportWithProgress(opts, progress)
}

func buildCropFilter(srcW, srcH int, ratio AspectRatio) string {
	var targetW, targetH int
	for _, opt := range AspectRatioOptions {
		if opt.Ratio == ratio {
			targetW, targetH = opt.W, opt.H
			break
		}
	}
	if targetW == 0 || targetH == 0 {
		return ""
	}

	srcRatio := float64(srcW) / float64(srcH)
	targetRatio := float64(targetW) / float64(targetH)

	var cropW, cropH int
	if srcRatio > targetRatio {
		cropH = srcH
		cropW = int(float64(srcH) * targetRatio)
	} else {
		cropW = srcW
		cropH = int(float64(srcW) / targetRatio)
	}

	// H.264 requires even dimensions
	cropW = cropW &^ 1
	cropH = cropH &^ 1

	return fmt.Sprintf("crop=%d:%d", cropW, cropH)
}

func generateOutputName(input string) string {
	dir := filepath.Dir(input)
	ext := filepath.Ext(input)
	base := strings.TrimSuffix(filepath.Base(input), ext)

	trimmedPath := filepath.Join(dir, base+"_trimmed"+ext)
	if !fileExists(trimmedPath) {
		return trimmedPath
	}

	for i := 1; i <= 999; i++ {
		numberedPath := filepath.Join(dir, fmt.Sprintf("%s_%03d%s", base, i, ext))
		if !fileExists(numberedPath) {
			return numberedPath
		}
	}

	return filepath.Join(dir, base+"_trimmed_new"+ext)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
