package ui

// Layout constants
const (
	minPanelWidth  = 10
	minPanelHeight = 5
	// Border (2) + padding (2 left + 2 right) = 6 horizontal overhead per panel
	horizontalOverhead = 6
	// Border (2) + title line (1) = 3 vertical overhead per panel
	verticalOverhead = 3
	// Timeline fixed height (includes border)
	// Content: time line + marker line + progress bar + cursor line + help = 5 lines
	// Plus vertical overhead (3) = 8
	timelineFixedHeight = 8
	// Properties panel fixed width
	propertiesFixedWidth = 30
)

// PanelDimensions holds the calculated dimensions for all panels
// Layout: Preview + Properties (top row, horizontal split) + Timeline (bottom, fixed height)
type PanelDimensions struct {
	// Total panel dimensions (for lipgloss Width/Height)
	PreviewWidth     int
	PreviewHeight    int
	PropertiesWidth  int
	PropertiesHeight int
	TimelineWidth    int
	TimelineHeight   int
	// Content dimensions (what gets passed to panel Render)
	PreviewContentWidth    int
	PreviewContentHeight   int
	PropertiesContentWidth  int
	PropertiesContentHeight int
	TimelineContentWidth   int
	TimelineContentHeight  int
}

// CalculatePanelDimensions calculates panel dimensions based on terminal size
// Layout: Preview + Properties (top row), Timeline (bottom, fixed height)
func CalculatePanelDimensions(termWidth, termHeight int) PanelDimensions {
	// Timeline has fixed height, top row takes the rest
	timelineHeight := timelineFixedHeight
	topRowHeight := termHeight - timelineHeight

	// Properties has fixed width, preview takes the rest
	propertiesWidth := propertiesFixedWidth
	previewWidth := termWidth - propertiesWidth

	return PanelDimensions{
		PreviewWidth:            previewWidth,
		PreviewHeight:           topRowHeight,
		PropertiesWidth:         propertiesWidth,
		PropertiesHeight:        topRowHeight,
		TimelineWidth:           termWidth,
		TimelineHeight:          timelineHeight,
		PreviewContentWidth:     max(0, previewWidth-horizontalOverhead),
		PreviewContentHeight:    max(0, topRowHeight-verticalOverhead),
		PropertiesContentWidth:  max(0, propertiesWidth-horizontalOverhead),
		PropertiesContentHeight: max(0, topRowHeight-verticalOverhead),
		TimelineContentWidth:    max(0, termWidth-horizontalOverhead),
		TimelineContentHeight:   max(0, timelineHeight-verticalOverhead),
	}
}
