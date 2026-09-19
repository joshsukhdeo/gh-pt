package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

type AssetInfo struct {
	Name        string
	FullName    string
	Symlink     string
	InstallCmd  string
	Completed   bool
}

type PacmanUI struct {
	Repo    string
	Version string
	Archive string
	Target  string

	Assets        []AssetInfo
	CurrentAsset  int
	DisableIcons  bool

	currentStage int
	dotsEaten    int
	paused       bool
	mu           sync.Mutex
	stopCh       chan struct{}
	animationDone chan struct{}
}

var GlobalPacman *PacmanUI

func NewPacmanUI(repo string) *PacmanUI {
	return &PacmanUI{
		Repo:          repo,
		Version:       "?",
		Archive:       "?",
		Target:        "?",
		stopCh:        make(chan struct{}),
		animationDone: make(chan struct{}),
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
	p.paused = false
	p.mu.Unlock()
	
	// Complete the animation for all assets
	p.completeAllAnimations()
	p.render()
	fmt.Println()
	
	// Signal that animation is done
	select {
	case <-p.animationDone:
		// Already closed
	default:
		close(p.animationDone)
	}
}

func (p *PacmanUI) WaitForAnimation() {
	<-p.animationDone
}

func (p *PacmanUI) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
	fmt.Print("\r\033[K")
}

func (p *PacmanUI) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = false
}

func (p *PacmanUI) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paused {
		return
	}
	
	if len(p.Assets) == 0 {
		// Single asset mode - legacy behavior
		if p.currentStage < 5 && p.dotsEaten < 3 {
			p.dotsEaten++
		}
	} else {
		// Multiple assets mode
		if p.CurrentAsset < len(p.Assets) {
			asset := &p.Assets[p.CurrentAsset]
			if !asset.Completed {
				if p.dotsEaten < 3 {
					p.dotsEaten++
				} else {
					// Move to next asset
					asset.Completed = true
					p.CurrentAsset++
					p.dotsEaten = 0
				}
			}
		}
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
	if target != "" {
		p.Target = target
	}
	
	// Handle single asset update (legacy)
	if asset != "" && len(p.Assets) == 0 {
		if len(p.Assets) == 0 {
			p.Assets = []AssetInfo{{Name: asset}}
		} else {
			p.Assets[0].Name = asset
		}
	}
}

func (p *PacmanUI) AddAsset(name, fullName, symlink, installCmd string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.Assets = append(p.Assets, AssetInfo{
		Name:       name,
		FullName:   fullName,
		Symlink:    symlink,
		InstallCmd: installCmd,
		Completed:  false,
	})
}

func (p *PacmanUI) completeAllAnimations() {
	p.currentStage = 5
	for i := range p.Assets {
		p.Assets[i].Completed = true
	}
	p.CurrentAsset = len(p.Assets)
}

func (p *PacmanUI) getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80 // default width
	}
	return width
}

func (p *PacmanUI) renderTo(sb *strings.Builder) {
	pacman := "\033[1;33mᗧ\033[0m"
	dot := " •"
	cherry := "🍒"
	flag := "🏁"
	link := "🔗"
	
	if p.DisableIcons {
		pacman = "\033[1;33mC\033[0m"
		dot = " ."
		cherry = "*"
		flag = "F"
		link = "L"
	}

	termWidth := p.getTerminalWidth()
	
	// Header line
	header := fmt.Sprintf("🐙 %s \033[1;33m🏷️\033[0m %s \033[1;33m📦\033[0m %s", 
		p.Repo, p.Version, p.Archive)
	if p.DisableIcons {
		header = fmt.Sprintf("R: %s V: %s A: %s", p.Repo, p.Version, p.Archive)
	}
	sb.WriteString(header + "\n")

	if len(p.Assets) == 0 {
		return
	}

	// Render each asset
	for i, asset := range p.Assets {
		if i > 0 {
			sb.WriteString("\n")
		}
		
		// Build the asset line
		var line strings.Builder
		
		// Asset name with cherry
		assetPart := fmt.Sprintf("%s %s", cherry, asset.Name)
		if p.DisableIcons {
			assetPart = fmt.Sprintf("%s %s", cherry, asset.Name)
		}
		
		// Finish flag with full name
		finishPart := ""
		if asset.Completed {
			if asset.FullName != "" {
				finishPart = fmt.Sprintf("%s %s", flag, asset.FullName)
			} else if asset.InstallCmd != "" {
				finishPart = fmt.Sprintf("%s %s", flag, asset.InstallCmd)
			}
			
			// Add symlink if present
			if asset.Symlink != "" {
				finishPart += fmt.Sprintf(" <--%s %s", link, asset.Symlink)
			}
		}
		
		// Calculate spacing
		prefix := ""
		if i < p.CurrentAsset {
			prefix = "    " // Completed assets
		} else if i == p.CurrentAsset {
			prefix = "" // Current asset with pacman
		} else {
			prefix = "    " // Future assets
		}
		
		line.WriteString(prefix)
		
		// Add dots and pacman
		if i < p.CurrentAsset {
			// Completed - show eaten dots
			line.WriteString("       ")
		} else if i == p.CurrentAsset {
			// Current - animate pacman
			eaten := p.dotsEaten
			if eaten > 3 {
				eaten = 3
			}
			line.WriteString(strings.Repeat("  ", eaten))
			line.WriteString(pacman)
			rem := 3 - eaten
			for j := 0; j < rem; j++ {
				line.WriteString(dot)
			}
			line.WriteString(" ")
		} else {
			// Future - show full dots
			line.WriteString(dot + dot + dot + " ")
		}
		
		// Add asset part
		line.WriteString(assetPart)
		
		// Add finish part if completed
		if asset.Completed && finishPart != "" {
			line.WriteString(" " + finishPart)
		}
		
		// Truncate if too long
		lineStr := line.String()
		if len(lineStr) > termWidth {
			lineStr = lineStr[:termWidth-3] + "..."
		}
		
		sb.WriteString(lineStr)
	}
}

func (p *PacmanUI) render() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paused {
		return
	}

	// Move cursor up to overwrite previous render
	if len(p.Assets) > 0 {
		fmt.Printf("\033[%dA", len(p.Assets)+1)
	}
	
	// Clear from cursor to end of screen
	fmt.Print("\033[J")

	var sb strings.Builder
	p.renderTo(&sb)
	fmt.Print(sb.String())
}

func (p *PacmanUI) SetCurrentAsset(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index >= 0 && index < len(p.Assets) {
		p.CurrentAsset = index
		p.dotsEaten = 0
	}
}

func (p *PacmanUI) UpdateSymlink(symlink string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Update symlink for current asset
	if len(p.Assets) > 0 && p.CurrentAsset < len(p.Assets) {
		p.Assets[p.CurrentAsset].Symlink = symlink
	}
}

func (p *PacmanUI) SetAssetInstallCmd(installCmd string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Update install command for current asset
	if len(p.Assets) > 0 && p.CurrentAsset < len(p.Assets) {
		p.Assets[p.CurrentAsset].InstallCmd = installCmd
	}
}

type PacmanLogWriter struct {
	io.Writer
}

func (w PacmanLogWriter) Write(p []byte) (int, error) {
	if GlobalPacman != nil {
		GlobalPacman.mu.Lock()
		defer GlobalPacman.mu.Unlock()
		if !GlobalPacman.paused {
			// Clear pacman lines
			if len(GlobalPacman.Assets) > 0 {
				fmt.Printf("\033[%dA\033[J", len(GlobalPacman.Assets)+1)
			} else {
				fmt.Print("\r\033[K")
			}
		}
	}
	n, err := w.Writer.Write(p)
	if GlobalPacman != nil && !GlobalPacman.paused {
		var sb strings.Builder
		GlobalPacman.renderTo(&sb)
		fmt.Print(sb.String())
	}
	return n, err
}
