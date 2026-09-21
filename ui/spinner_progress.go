package ui

import (
	bubblesSpinner "charm.land/bubbles/v2/spinner"
)

// SpinnerProgressBar uses bubbles/spinner for animated spinner progress.
type SpinnerProgressBar struct {
	model   bubblesSpinner.Model
	style   string
	started bool
}

// Valid spinner styles
var validSpinnerStyles = map[string]bubblesSpinner.Spinner{
	"dots":      bubblesSpinner.Dot,
	"line":      bubblesSpinner.Line,
	"jump":      bubblesSpinner.Jump,
	"pulse":     bubblesSpinner.Pulse,
	"points":    bubblesSpinner.Points,
	"miniDot":   bubblesSpinner.MiniDot,
	"step":      bubblesSpinner.Ellipsis, // Ellipsis is closest to "step" style
	"globe":     bubblesSpinner.Globe,
	"moon":      bubblesSpinner.Moon,
	"monkey":    bubblesSpinner.Monkey,
	"meter":     bubblesSpinner.Meter,
	"hamburger": bubblesSpinner.Hamburger,
	"ellipsis":  bubblesSpinner.Ellipsis,
}

func NewSpinnerProgressBar(style string) *SpinnerProgressBar {
	s, ok := validSpinnerStyles[style]
	if !ok {
		s = bubblesSpinner.Dot // default
	}
	return &SpinnerProgressBar{
		model: bubblesSpinner.New(bubblesSpinner.WithSpinner(s)),
		style: style,
	}
}

func (p *SpinnerProgressBar) Start(total int) {
	p.started = true
}

func (p *SpinnerProgressBar) Update(current int) {
	// Spinner doesn't use current/total, just animates
}

func (p *SpinnerProgressBar) Finish() {
	p.started = false
}

func (p *SpinnerProgressBar) SetAsset(name, fullName, symlink, installCmd string)           {}
func (p *SpinnerProgressBar) SetCurrentAsset(index int)                                     {}
func (p *SpinnerProgressBar) SetAssetInstallCmd(installCmd string)                          {}
func (p *SpinnerProgressBar) UpdateSymlink(symlink string)                                  {}
func (p *SpinnerProgressBar) UpdateStage(stage int, version, archive, asset, target string) {}

func (p *SpinnerProgressBar) View() string {
	if !p.started {
		return ""
	}
	return p.model.View()
}

// Tick advances the spinner animation.
func (p *SpinnerProgressBar) Tick() {
	p.model.Tick()
}
