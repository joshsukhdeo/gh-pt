package ui

import (
	"fmt"

	bubblesProgress "charm.land/bubbles/v2/progress"
)

// StandardProgressBar uses bubbles/progress for a traditional progress bar.
type StandardProgressBar struct {
	model   bubblesProgress.Model
	total   int
	current int
}

func NewStandardProgressBar() *StandardProgressBar {
	return &StandardProgressBar{
		model: bubblesProgress.New(
			bubblesProgress.WithDefaultBlend(),
			bubblesProgress.WithoutPercentage(),
		),
	}
}

func (p *StandardProgressBar) Start(total int) {
	p.total = total
	p.current = 0
}

func (p *StandardProgressBar) Update(current int) {
	p.current = current
}

func (p *StandardProgressBar) Finish() {
	p.current = p.total
}

func (p *StandardProgressBar) SetAsset(name, fullName, symlink, installCmd string)           {}
func (p *StandardProgressBar) SetCurrentAsset(index int)                                     {}
func (p *StandardProgressBar) SetAssetInstallCmd(installCmd string)                          {}
func (p *StandardProgressBar) UpdateSymlink(symlink string)                                  {}
func (p *StandardProgressBar) UpdateStage(stage int, version, archive, asset, target string) {}

func (p *StandardProgressBar) View() string {
	if p.total == 0 {
		return ""
	}
	percent := float64(p.current) / float64(p.total)
	if percent > 1 {
		percent = 1
	}
	return fmt.Sprintf("%s %d%%", p.model.ViewAs(percent), int(percent*100))
}
