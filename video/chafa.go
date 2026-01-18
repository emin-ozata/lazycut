package video

import (
	"fmt"
	"strconv"
)

type QualityPreset int

const (
	QualityLow QualityPreset = iota
	QualityMedium
	QualityHigh
)

func (q QualityPreset) String() string {
	switch q {
	case QualityLow:
		return "LOW"
	case QualityMedium:
		return "MEDIUM"
	case QualityHigh:
		return "HIGH"
	}
	return "UNKNOWN"
}

func (q QualityPreset) Next() QualityPreset {
	return (q + 1) % 3
}

type ChafaConfig struct {
	Colors         string
	Optimize       int
	Work           int
	ColorSpace     string
	Dither         string
	ColorExtractor string
}

var ChafaPresets = map[QualityPreset]ChafaConfig{
	QualityLow: {
		Colors: "256", Optimize: 9, Work: 1,
		ColorSpace: "rgb", Dither: "none", ColorExtractor: "average",
	},
	QualityMedium: {
		Colors: "256", Optimize: 5, Work: 5,
		ColorSpace: "rgb", Dither: "ordered", ColorExtractor: "average",
	},
	QualityHigh: {
		Colors: "full", Optimize: 3, Work: 9,
		ColorSpace: "din99d", Dither: "diffusion", ColorExtractor: "median",
	},
}

func (c ChafaConfig) BuildArgs(width, height int) []string {
	return []string{
		"--format=symbols",
		"--size", fmt.Sprintf("%dx%d", width, height),
		"--colors", c.Colors,
		"-O", strconv.Itoa(c.Optimize),
		"--work", strconv.Itoa(c.Work),
		"--color-space", c.ColorSpace,
		"--dither", c.Dither,
		"--color-extractor", c.ColorExtractor,
		"-",
	}
}
