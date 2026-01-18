package panels

import (
	"fmt"
	"lazycut/video"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type Timeline struct {
	player       *video.Player
	exportStatus string
}

func NewTimeline(player *video.Player) *Timeline {
	return &Timeline{
		player: player,
	}
}

func (t *Timeline) SetExportStatus(status string) {
	t.exportStatus = status
}

func (t *Timeline) Render(width, height int) string {
	pos := t.player.Position()
	dur := t.player.Duration()
	playing := t.player.IsPlaying()
	trim := &t.player.Trim

	posStr := formatDuration(pos)
	durStr := formatDuration(dur)

	playIcon := "▶ "
	if playing {
		playIcon = "❚❚"
	}

	barWidth := width - 3
	if barWidth < 10 {
		barWidth = 10
	}

	line1 := fmt.Sprintf(" %s %s / %s", playIcon, posStr, durStr)
	line2 := " " + t.buildMarkerLine(barWidth, dur, trim)
	line3 := " " + t.buildProgressBar(barWidth, pos, dur, trim)
	line4 := " " + t.buildCursorLine(barWidth, pos, dur)

    var line5 string
    if t.exportStatus != "" {
        line5 = " " + t.exportStatus
    } else if trim.IsComplete() {
        trimDur := formatDuration(trim.Duration())
        line5 = fmt.Sprintf(" [%s] Enter:export  p:preview  d/Esc:clear  h/l:±1s  H/L:±5s  ,/.:±1f  0:home  G/$:end  ?:help", trimDur)
    } else if trim.InPoint != nil {
        line5 = " IN set | o:set out  d/Esc:clear  h/l:±1s  H/L:±5s  ,/.:±1f  0:home  G/$:end  ?:help"
    } else if trim.OutPoint != nil {
        line5 = " OUT set | i:set in  d/Esc:clear  h/l:±1s  H/L:±5s  ,/.:±1f  0:home  G/$:end  ?:help"
    } else {
        line5 = " h/l:±1s  H/L:±5s  ,/.:±1f  i:in  o:out  m:mute  Tab:quality  0:home  G/$:end  ?:help"
    }

	content := strings.Join([]string{line1, line2, line3, line4, line5}, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

func (t *Timeline) buildProgressBar(barWidth int, pos, dur time.Duration, trim *video.TrimState) string {
	if dur <= 0 {
		return "[" + repeat("-", barWidth) + "]"
	}

	posIdx := int(float64(pos) / float64(dur) * float64(barWidth))
	if posIdx > barWidth {
		posIdx = barWidth
	}

	var inIdx, outIdx int = -1, -1
	if trim.InPoint != nil {
		inIdx = int(float64(*trim.InPoint) / float64(dur) * float64(barWidth))
		if inIdx > barWidth {
			inIdx = barWidth
		}
	}
	if trim.OutPoint != nil {
		outIdx = int(float64(*trim.OutPoint) / float64(dur) * float64(barWidth))
		if outIdx > barWidth {
			outIdx = barWidth
		}
	}

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		inSelection := false
		if inIdx >= 0 && outIdx >= 0 && i >= inIdx && i <= outIdx {
			inSelection = true
		}

		if inSelection {
			bar.WriteString("▓")
		} else if i < posIdx {
			bar.WriteString("=")
		} else {
			bar.WriteString("-")
		}
	}
	bar.WriteString("]")

	return bar.String()
}

func (t *Timeline) buildMarkerLine(barWidth int, dur time.Duration, trim *video.TrimState) string {
	if dur <= 0 {
		return repeat(" ", barWidth+2)
	}

	inStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	outStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("166")).Bold(true)

	line := make([]string, barWidth+2)
	for i := range line {
		line[i] = " "
	}

	if trim.InPoint != nil {
		inIdx := int(float64(*trim.InPoint)/float64(dur)*float64(barWidth)) + 1
		if inIdx >= len(line) {
			inIdx = len(line) - 1
		}
		line[inIdx] = inStyle.Render("▼")
	}

	if trim.OutPoint != nil {
		outIdx := int(float64(*trim.OutPoint)/float64(dur)*float64(barWidth)) + 1
		if outIdx >= len(line) {
			outIdx = len(line) - 1
		}
		line[outIdx] = outStyle.Render("▼")
	}

	return strings.Join(line, "")
}

func (t *Timeline) buildCursorLine(barWidth int, pos, dur time.Duration) string {
	if dur <= 0 {
		return repeat(" ", barWidth+2)
	}

	line := make([]rune, barWidth+2)
	for i := range line {
		line[i] = ' '
	}

	posIdx := int(float64(pos)/float64(dur)*float64(barWidth)) + 1
	if posIdx >= len(line) {
		posIdx = len(line) - 1
	}
	line[posIdx] = '▲'

	return string(line)
}

func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	mins := total / 60
	secs := total % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
