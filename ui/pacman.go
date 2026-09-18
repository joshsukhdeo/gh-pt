package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type PacmanUI struct {
	Repo    string
	Version string
	Archive string
	Asset   string
	Target  string

	GhostType string
	Symlink   string

	DisableIcons bool

	currentStage int
	dotsEaten    int
	paused       bool
	mu           sync.Mutex
	stopCh       chan struct{}
}

var GlobalPacman *PacmanUI

func NewPacmanUI(repo string) *PacmanUI {
	return &PacmanUI{
		Repo:      repo,
		Version:   "?",
		Archive:   "?",
		Asset:     "?",
		Target:    "?",
		GhostType: "🍒",
		stopCh:    make(chan struct{}),
	}
}

func (p *PacmanUI) Start() {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.tick()
				p.render()
			}
		}
	}()
}

func (p *PacmanUI) Stop() {
	close(p.stopCh)
	p.mu.Lock()
	p.currentStage = 5
	p.dotsEaten = 3
	p.paused = false
	p.mu.Unlock()
	p.render()
	fmt.Println()
}

func (p *PacmanUI) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
	fmt.Print("\r\033[K")
}
func (p *PacmanUI) Resume() { p.mu.Lock(); defer p.mu.Unlock(); p.paused = false }

func (p *PacmanUI) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paused {
		return
	}
	if p.currentStage < 5 && p.dotsEaten < 3 {
		p.dotsEaten++
	}
}

func (p *PacmanUI) Update(stage int, version, archive, asset, target, ghostType string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if stage > p.currentStage {
		p.currentStage = stage
		p.dotsEaten = 0
	}

	if version != "" {
		p.Version = version
	}
	if archive != "" {
		p.Archive = archive
	}
	if asset != "" {
		p.Asset = asset
	}
	if target != "" {
		p.Target = target
	}
	if ghostType != "" {
		p.GhostType = ghostType
	}
}

func (p *PacmanUI) renderTo(sb *strings.Builder) {
	nodes := []string{
		"", // start
		p.Repo,
		p.Version,
		p.Archive,
		p.Asset,
		p.Target,
	}

	if !p.DisableIcons {
		nodes[1] = "🐙 " + p.Repo
		nodes[2] = "🏷️ " + p.Version
		nodes[3] = "📦 " + p.Archive
		nodes[4] = p.GhostType + " " + p.Asset
		nodes[5] = "🏁 " + p.Target
	}

	if p.Symlink != "" {
		if !p.DisableIcons {
			nodes = append(nodes, "🔗 "+p.Symlink)
		} else {
			nodes = append(nodes, p.Symlink)
		}
	}

	pacman := "\033[1;33mᗧ\033[0m" // Yellow pacman
	dot := " •"
	if p.DisableIcons {
		pacman = "\033[1;33mC\033[0m"
		dot = " ."
	}

	maxStage := len(nodes) - 1

	for i := 1; i <= maxStage; i++ {
		// Print previous node
		if nodes[i-1] != "" {
			sb.WriteString(nodes[i-1] + " ")
		}

		if i < p.currentStage+1 {
			// Pacman passed this segment, draw empty spaces where dots were
			sb.WriteString("       ") // 7 spaces for " • • • "
		} else if i == p.currentStage+1 {
			// Pacman is in this segment
			eaten := p.dotsEaten
			if eaten > 3 {
				eaten = 3
			}

			// spaces for eaten dots
			sb.WriteString(strings.Repeat("  ", eaten))

			// draw pacman
			sb.WriteString(pacman)

			// draw remaining dots
			rem := 3 - eaten
			for j := 0; j < rem; j++ {
				sb.WriteString(dot)
			}
			sb.WriteString(" ")
		} else {
			// Pacman hasn't reached here
			sb.WriteString(dot + dot + dot + " ")
		}
	}

	// Final node
	if p.currentStage >= maxStage {
		sb.WriteString(nodes[maxStage] + " " + pacman)
	} else {
		sb.WriteString(nodes[maxStage])
	}
}

func (p *PacmanUI) render() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paused {
		return
	}

	var sb strings.Builder
	sb.WriteString("\r\033[K") // clear line
	p.renderTo(&sb)
	fmt.Print(sb.String())
}

func (p *PacmanUI) UpdateSymlink(symlink string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Symlink = symlink
}

type PacmanLogWriter struct {
	io.Writer
}

func (w PacmanLogWriter) Write(p []byte) (int, error) {
	if GlobalPacman != nil {
		GlobalPacman.mu.Lock()
		defer GlobalPacman.mu.Unlock()
		if !GlobalPacman.paused {
			_, _ = fmt.Fprint(w.Writer, "\r\033[K")
		}
	}
	n, err := w.Writer.Write(p)
	if GlobalPacman != nil && !GlobalPacman.paused {
		var sb strings.Builder
		GlobalPacman.renderTo(&sb)
		_, _ = fmt.Fprint(w.Writer, sb.String())
	}
	return n, err
}
