package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type PacmanUI struct {
	Repo         string
	Version      string
	Archive      string
	Asset        string
	Target       string
	
	GhostType    string
	Symlink      string
	
	currentStage int
	dotsEaten    int
	paused       bool
	mu           sync.Mutex
	stopCh       chan struct{}
}

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
	
	if version != "" { p.Version = version }
	if archive != "" { p.Archive = archive }
	if asset != "" { p.Asset = asset }
	if target != "" { p.Target = target }
	if ghostType != "" { p.GhostType = ghostType }
}

func (p *PacmanUI) render() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paused {
		return
	}

	var sb strings.Builder
	sb.WriteString("\r\033[K") // clear line

	nodes := []string{
		"", // start
		"🐙 " + p.Repo,
		"🏷️ " + p.Version,
		"📦 " + p.Archive,
		p.GhostType + " " + p.Asset,
		"🏁 " + p.Target,
	}
	
	if p.Symlink != "" {
		nodes = append(nodes, "🔗 " + p.Symlink)
	}

	pacman := "\033[1;33mᗧ\033[0m" // Yellow pacman

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
			if eaten > 3 { eaten = 3 }
			
			// spaces for eaten dots
			sb.WriteString(strings.Repeat("  ", eaten))
			
			// draw pacman
			sb.WriteString(pacman)
			
			// draw remaining dots
			rem := 3 - eaten
			for j := 0; j < rem; j++ {
				sb.WriteString(" •")
			}
			sb.WriteString(" ")
		} else {
			// Pacman hasn't reached here
			sb.WriteString(" • • • ")
		}
	}
	
	// Final node
	if p.currentStage >= maxStage {
		sb.WriteString(nodes[maxStage] + " " + pacman)
	} else {
		sb.WriteString(nodes[maxStage])
	}

	fmt.Print(sb.String())
}

func (p *PacmanUI) UpdateSymlink(symlink string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Symlink = symlink
}
