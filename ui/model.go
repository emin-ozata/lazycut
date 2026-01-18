package ui

import (
	"fmt"
	"lazycut/ui/panels"
	"lazycut/video"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	PanelPreview = iota
	PanelTimeline
)

type TickMsg time.Time

type ExportDoneMsg struct {
	Output string
	Err    error
}

type ExportProgressMsg float64

type Model struct {
	width        int
	height       int
	player       *video.Player
	preview      *panels.Preview
	properties   *panels.Properties
	timeline     *panels.Timeline
	ready        bool
	previewMode  bool
	exportStatus string

	showExportModal    bool
	exportFilename     string
	exportAspectRatio  int // index into video.AspectRatioOptions
	exportFocusField   int // 0: filename, 1: aspect ratio
	exporting          bool
	exportProgress     float64
	exportProgressChan <-chan float64

    showHelpModal bool
    undoStack     []trimSnapshot

    // Vim-style input
    repeatCount int
}

type trimSnapshot struct {
	inPoint  *time.Duration
	outPoint *time.Duration
}

func NewModel(player *video.Player) Model {
	return Model{
		player:     player,
		preview:    panels.NewPreview(player),
		properties: panels.NewProperties(player),
		timeline:   panels.NewTimeline(player),
		ready:      false,
	}
}

func (m *Model) saveTrimState() {
	snapshot := trimSnapshot{}
	if m.player.Trim.InPoint != nil {
		val := *m.player.Trim.InPoint
		snapshot.inPoint = &val
	}
	if m.player.Trim.OutPoint != nil {
		val := *m.player.Trim.OutPoint
		snapshot.outPoint = &val
	}
	m.undoStack = append(m.undoStack, snapshot)
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ExportProgressMsg:
		m.exportProgress = float64(msg)
		if m.exportProgressChan != nil {
			return m, listenProgress(m.exportProgressChan)
		}
		return m, nil

	case ExportDoneMsg:
		m.exporting = false
		m.showExportModal = false
		m.exportProgress = 0
		m.exportProgressChan = nil
		if msg.Err != nil {
			m.exportStatus = "Export failed: " + msg.Err.Error()
		} else {
			m.exportStatus = "Exported: " + msg.Output
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		dims := CalculatePanelDimensions(m.width, m.height)
		m.player.SetSize(dims.PreviewContentWidth, dims.PreviewContentHeight)
		return m, nil

	case TickMsg:
		if m.previewMode && m.player.IsPlaying() {
			if m.player.Trim.OutPoint != nil && m.player.Position() >= *m.player.Trim.OutPoint {
				m.player.Pause()
				m.previewMode = false
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		if m.showHelpModal {
			return m.handleHelpModalKey(msg)
		}
		if m.showExportModal {
			return m.handleExportModalKey(msg)
		}
		m.exportStatus = ""

		pos := m.player.Position()
		fps := m.player.FPS()
		frameDuration := time.Second / time.Duration(fps)

		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.repeatCount = m.repeatCount*10 + int(msg.Runes[0]-'0')
			m.exportStatus = fmt.Sprintf("%dx", m.repeatCount)
			return m, nil
		case "0":
			if m.repeatCount == 0 {
				m.player.Seek(0)
				return m, nil
			}
			m.repeatCount *= 10
			m.exportStatus = fmt.Sprintf("%dx", m.repeatCount)
			return m, nil
		case "ctrl+c", "q":
			m.player.Close()
			return m, tea.Quit

		case " ":
			m.player.Toggle()
			return m, nil

		case "h":
			n := m.repeatCount
			if n <= 0 { n = 1 }
			m.player.Seek(pos - time.Duration(n)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "l":
			n := m.repeatCount
			if n <= 0 { n = 1 }
			m.player.Seek(pos + time.Duration(n)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "H":
			n := m.repeatCount
			if n <= 0 { n = 1 }
			m.player.Seek(pos - time.Duration(n*5)*time.Second)
			m.repeatCount = 0
			return m, nil

		case "L":
			n := m.repeatCount
			if n <= 0 { n = 1 }
			m.player.Seek(pos + time.Duration(n*5)*time.Second)
			m.repeatCount = 0
			return m, nil

		case ",":
			n := m.repeatCount
			if n <= 0 { n = 1 }
			m.player.Seek(pos - time.Duration(n)*frameDuration)
			m.repeatCount = 0
			return m, nil

		case ".":
			n := m.repeatCount
			if n <= 0 { n = 1 }
			m.player.Seek(pos + time.Duration(n)*frameDuration)
			m.repeatCount = 0
			return m, nil

		case "$", "G":
			m.player.Seek(m.player.Duration())
			m.repeatCount = 0
			return m, nil

		case "i":
			m.saveTrimState()
			m.player.Trim.SetIn(pos)
			return m, nil

		case "o":
			m.saveTrimState()
			m.player.Trim.SetOut(pos)
			return m, nil

		case "p":
			if m.player.Trim.InPoint != nil {
				m.player.Seek(*m.player.Trim.InPoint)
				m.previewMode = true
				m.player.Play()
			}
			return m, nil

		case "enter":
			if m.player.Trim.IsComplete() {
				m.showExportModal = true
				m.exportFilename = ""
				m.exportAspectRatio = 0
			}
			return m, nil

		case "esc", "d":
			if m.player.Trim.InPoint != nil || m.player.Trim.OutPoint != nil {
				m.saveTrimState()
			}
			m.player.Trim.Clear()
			m.previewMode = false
			return m, nil

		case "?":
			m.showHelpModal = true
			return m, nil

		case "u":
			if len(m.undoStack) > 0 {
				last := m.undoStack[len(m.undoStack)-1]
				m.undoStack = m.undoStack[:len(m.undoStack)-1]
				m.player.Trim.InPoint = last.inPoint
				m.player.Trim.OutPoint = last.outPoint
			}
			return m, nil

		case "tab":
			m.player.CycleQuality()
			return m, nil
		}
	}

	return m, nil
}

func renderPanel(content, title string, width, height int) string {
    innerWidth := width - 2
    innerHeight := height - 2

    // Combine title and content only if title provided
    inner := content
    if strings.TrimSpace(title) != "" {
        inner = title + "\n" + content
    }
	lines := strings.Split(inner, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	paddedContent := strings.Join(lines[:innerHeight], "\n")

	return BorderStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(paddedContent)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	dims := CalculatePanelDimensions(m.width, m.height)

	if dims.PreviewContentWidth < minPanelWidth || dims.PreviewContentHeight < minPanelHeight {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Terminal too small")
	}

    previewContent := m.preview.Render(dims.PreviewContentWidth, dims.PreviewContentHeight)
    previewPanel := renderPanel(previewContent, "", dims.PreviewWidth, dims.PreviewHeight)

    propertiesContent := m.properties.Render(dims.PropertiesContentWidth, dims.PropertiesContentHeight)
    propertiesPanel := renderPanel(propertiesContent, "", dims.PropertiesWidth, dims.PropertiesHeight)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, previewPanel, propertiesPanel)

    m.timeline.SetExportStatus(m.exportStatus)
    timelineContent := m.timeline.Render(dims.TimelineContentWidth, dims.TimelineContentHeight)
    timelinePanel := renderPanel(timelineContent, "", dims.TimelineWidth, dims.TimelineHeight)

	base := lipgloss.JoinVertical(lipgloss.Left, topRow, timelinePanel)

	if m.showHelpModal {
		return m.renderHelpModal(base)
	}
	if m.showExportModal {
		return m.renderExportModal(base)
	}

	return base
}

func (m Model) handleExportModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.Type {
	case tea.KeyEsc:
		if !m.exporting {
			m.showExportModal = false
		}
		return m, nil

	case tea.KeyEnter:
		if m.exporting {
			return m, nil
		}
		m.exporting = true
		m.exportProgress = 0
		progressChan := make(chan float64, 100)
		m.exportProgressChan = progressChan
		props := m.player.Properties()
		opts := video.ExportOptions{
			Input:       m.player.Path(),
			Output:      m.exportFilename,
			InPoint:     *m.player.Trim.InPoint,
			OutPoint:    *m.player.Trim.OutPoint,
			AspectRatio: video.AspectRatioOptions[m.exportAspectRatio].Ratio,
			Width:       props.Width,
			Height:      props.Height,
		}
		return m, startExportWithChan(opts, progressChan)

	case tea.KeyUp, tea.KeyShiftTab:
		if m.exportFocusField > 0 {
			m.exportFocusField--
		}
		return m, nil

	case tea.KeyDown, tea.KeyTab:
		if m.exportFocusField < 1 {
			m.exportFocusField++
		}
		return m, nil

	case tea.KeyLeft:
		if m.exportFocusField == 1 {
			m.exportAspectRatio--
			if m.exportAspectRatio < 0 {
				m.exportAspectRatio = len(video.AspectRatioOptions) - 1
			}
		}
		return m, nil

	case tea.KeyRight:
		if m.exportFocusField == 1 {
			m.exportAspectRatio = (m.exportAspectRatio + 1) % len(video.AspectRatioOptions)
		}
		return m, nil

	case tea.KeyBackspace:
		if m.exportFocusField == 0 && len(m.exportFilename) > 0 {
			m.exportFilename = m.exportFilename[:len(m.exportFilename)-1]
		}
		return m, nil

    default:
        // Vim-style navigation aliases in modal
        switch msg.String() {
        case "j":
            if m.exportFocusField < 1 { m.exportFocusField++ }
            return m, nil
        case "k":
            if m.exportFocusField > 0 { m.exportFocusField-- }
            return m, nil
        case "h":
            if m.exportFocusField == 1 {
                m.exportAspectRatio--
                if m.exportAspectRatio < 0 {
                    m.exportAspectRatio = len(video.AspectRatioOptions) - 1
                }
            }
            return m, nil
        case "l":
            if m.exportFocusField == 1 {
                m.exportAspectRatio = (m.exportAspectRatio + 1) % len(video.AspectRatioOptions)
            }
            return m, nil
        }
        if m.exportFocusField == 0 && len(msg.Runes) > 0 {
            m.exportFilename += string(msg.Runes)
        }
        return m, nil
    }

	return m, nil
}

func (m Model) handleHelpModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q", "enter", " ":
		m.showHelpModal = false
		return m, nil
	}
	return m, nil
}

func (m Model) renderHelpModal(_ string) string {
	content := `PLAYBACK
Space     Play/Pause
m         Mute/Unmute
h / l     Seek ±1 second
H / L     Seek ±5 seconds
, / .     Seek ±1 frame
0         Go to start
G or $    Go to end
Counts    e.g. 5l, 10., 2H
Tab       Cycle quality

TRIM
i         Set in-point
o         Set out-point
p         Preview selection
d / Esc   Clear selection
Enter     Export (when selection set)

OTHER
u         Undo
?         Toggle help
q         Quit

[?] or [Esc] to close`

	title := lipgloss.NewStyle().
		Bold(true).
		Render("Help")

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 3).
		Width(45).
		Render(title + "\n\n" + content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func startExportWithChan(opts video.ExportOptions, progressChan chan float64) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			output, err := video.ExportWithProgress(opts, progressChan)
			return ExportDoneMsg{Output: output, Err: err}
		},
		listenProgress(progressChan),
	)
}

func listenProgress(ch <-chan float64) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return ExportProgressMsg(p)
	}
}

func (m Model) renderExportModal(_ string) string {
	var content string

	props := m.player.Properties()
	opts := video.ExportOptions{
		Input:       m.player.Path(),
		Output:      m.exportFilename,
		InPoint:     *m.player.Trim.InPoint,
		OutPoint:    *m.player.Trim.OutPoint,
		AspectRatio: video.AspectRatioOptions[m.exportAspectRatio].Ratio,
		Width:       props.Width,
		Height:      props.Height,
	}
	ffmpegCmd := video.BuildFFmpegCommand(opts)

	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)

	if m.exporting {
		title := lipgloss.NewStyle().
			Bold(true).
			Render("Exporting...")

		barWidth := 50
		filled := int(m.exportProgress * float64(barWidth))
		empty := barWidth - filled
		bar := "[" + strings.Repeat("=", filled) + strings.Repeat("-", empty) + "]"
		percent := fmt.Sprintf("%3.0f%%", m.exportProgress*100)

		content = fmt.Sprintf(`%s

%s %s

%s`, title, bar, percent, cmdStyle.Render(ffmpegCmd))
	} else {
		filename := m.exportFilename
		filenameDisplay := filename
		if m.exportFocusField == 0 {
			filenameDisplay = filename + "_"
		}
		if filename == "" && m.exportFocusField != 0 {
			filenameDisplay = "(auto)"
		}

		fnIndicator := "  "
		arIndicator := "  "
		if m.exportFocusField == 0 {
			fnIndicator = "> "
		} else {
			arIndicator = "> "
		}

		var ratioLine string
		for i, opt := range video.AspectRatioOptions {
			if i == m.exportAspectRatio {
				ratioLine += "[" + opt.Label + "] "
			} else {
				ratioLine += " " + opt.Label + "  "
			}
		}

		title := lipgloss.NewStyle().
			Bold(true).
			Render("Export Selection")

		content = fmt.Sprintf(`%s

%sFilename: %s

%sAspect:   %s

%s

[up/down or j/k]: switch field
[left/right or h/l]: change ratio
[enter]: export       [esc]: cancel`, title, fnIndicator, filenameDisplay, arIndicator, ratioLine, cmdStyle.Render(ffmpegCmd))
	}

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 3).
		Width(75).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
