package ui

import (
	"fmt"
	"io"
	"log/slog"
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

	// Animation phases
	// Phase 0: Header animation (eating through repo, version, archive)
	// Phase 1: Horizontal movement (moving right, wrapping at edge)
	// Phase 2: Asset resolution (showing actual asset info)
	phase         int
	headerEaten   int // How many header elements eaten (0-3: repo, version, archive, cherry)
	horizontalPos int // Position for horizontal movement
	wrapLine      int // Current wrap line number

	dotsEaten     int
	paused        bool
	mu            sync.Mutex
	stopCh        chan struct{}
	animationDone chan struct{}
	
	// Terminal state tracking
	headerPrinted bool
	lastLineCount int
}

var GlobalPacman *PacmanUI

func NewPacmanUI(repo string) *PacmanUI {
	return &PacmanUI{
		Repo:          repo,
		Version:       "?",
		Archive:       "?",
		Target:        "?",
		phase:         0,
		stopCh:        make(chan struct{}),
		animationDone: make(chan struct{}),
	}
}

func (p *PacmanUI) Start() {
	slog.Debug("pacman animation starting",
		"repo", p.Repo,
		"version", p.Version,
		"archive", p.Archive,
		"tty", term.IsTerminal(int(os.Stdout.Fd())),
	)
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
	slog.Info("pacman animation stopping", "assets_completed", p.CurrentAsset, "total_assets", len(p.Assets))
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

	switch p.phase {
	case 0: // Header animation
		if p.headerEaten < 4 { // repo, version, archive, cherry
			p.headerEaten++
		} else {
			// Move to phase 1
			p.phase = 1
			p.horizontalPos = 0
			p.wrapLine = 0
			slog.Debug("pacman phase transition", "from", 0, "to", 1, "reason", "header_complete")
		}
	case 1: // Horizontal movement
		p.horizontalPos++
		termWidth := p.getTerminalWidth()
		if p.horizontalPos >= termWidth {
			p.horizontalPos = 0
			p.wrapLine++
		}
		// Check if assets are resolved
		if len(p.Assets) > 0 && p.Assets[0].Name != "?" {
			p.phase = 2
			p.CurrentAsset = 0
			p.dotsEaten = 0
			slog.Debug("pacman phase transition", "from", 1, "to", 2, "reason", "assets_resolved", "asset_count", len(p.Assets))
		}
	case 2: // Asset resolution
		if len(p.Assets) == 0 {
			return
		}
		if p.CurrentAsset < len(p.Assets) {
			asset := &p.Assets[p.CurrentAsset]
			if !asset.Completed {
				if p.dotsEaten < 3 {
					p.dotsEaten++
				} else {
					// Move to next asset
					asset.Completed = true
					slog.Debug("pacman asset completed", "index", p.CurrentAsset, "name", asset.Name, "total", len(p.Assets))
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
		p.Assets = []AssetInfo{{Name: asset}}
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
	slog.Debug("pacman asset added", "name", name, "total", len(p.Assets))
}

func (p *PacmanUI) completeAllAnimations() {
	p.phase = 2
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

func (p *PacmanUI) renderAssets(sb *strings.Builder) {
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

	switch p.phase {
	case 0: // Header animation - pacman eating through elements
		p.renderPhase0(sb, pacman, dot, cherry, termWidth)
	case 1: // Horizontal movement - pacman moving right
		p.renderPhase1(sb, pacman, dot, cherry, termWidth)
	case 2: // Asset resolution - show actual assets
		p.renderPhase2(sb, pacman, dot, cherry, flag, link, termWidth)
	}
}

func (p *PacmanUI) renderPhase0(sb *strings.Builder, pacman, dot, cherry string, termWidth int) {
	// Build header with pacman eating through elements
	var line strings.Builder

	// Start with pacman
	line.WriteString(pacman)

	// Elements to eat through
	elements := []struct {
		icon string
		text string
	}{
		{"🐙", p.Repo},
		{"🏷️", p.Version},
		{"📦", p.Archive},
		{cherry, "?"},
	}

	if p.DisableIcons {
		elements[0].icon = "R:"
		elements[1].icon = "V:"
		elements[2].icon = "A:"
	}

	for i, elem := range elements {
		// Add dots before element (if not first)
		if i > 0 {
			line.WriteString(" " + dot + " " + dot + " " + dot + " ")
		}

		// Check if pacman has eaten to this element
		if i < p.headerEaten {
			// Element is visible
			line.WriteString(elem.icon + " " + elem.text)
		} else if i == p.headerEaten {
			// Pacman is eating dots before this element
			// Show pacman and remaining dots
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
			line.WriteString(elem.icon + " " + elem.text)
			break
		} else {
			// Element not reached yet
			line.WriteString(dot + " " + dot + " " + dot + " ")
			line.WriteString(elem.icon + " " + elem.text)
		}
	}

	// Add final unknown element
	if p.headerEaten >= 4 {
		line.WriteString(" " + dot + " " + dot + " " + dot + " ")
		line.WriteString("?")
	}

	lineStr := line.String()
	if len(lineStr) > termWidth {
		lineStr = lineStr[:termWidth-3] + "..."
	}
	sb.WriteString(lineStr)
}

func (p *PacmanUI) renderPhase1(sb *strings.Builder, pacman, dot, cherry string, termWidth int) {
	// Pacman moving horizontally
	var line strings.Builder

	// Add spaces for horizontal position
	for i := 0; i < p.horizontalPos; i++ {
		if i%2 == 0 {
			line.WriteString(" ")
		} else {
			line.WriteString(dot)
		}
	}

	// Add pacman
	line.WriteString(pacman)

	// Add remaining dots to fill line
	remaining := termWidth - p.horizontalPos - 2
	for i := 0; i < remaining && i < 20; i++ {
		if i%2 == 0 {
			line.WriteString(dot)
		} else {
			line.WriteString(" ")
		}
	}

	// Add cherry and unknown
	line.WriteString(" " + cherry + " ?")

	lineStr := line.String()
	if len(lineStr) > termWidth {
		lineStr = lineStr[:termWidth]
	}
	sb.WriteString(lineStr)
}

func (p *PacmanUI) renderPhase2(sb *strings.Builder, pacman, dot, cherry, flag, link string, termWidth int) {
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

		// Finish flag with full name or install command
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

		// Calculate spacing based on asset state
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

	// Check if stdout is a TTY - if not, disable animation
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		// Non-interactive mode: just print without cursor control
		var sb strings.Builder
		if !p.headerPrinted {
			p.renderHeader(&sb)
			sb.WriteString("\n")
			p.headerPrinted = true
			slog.Debug("pacman header printed (non-TTY mode)")
		}
		p.renderAssets(&sb)
		fmt.Println(sb.String())
		return
	}

	var sb strings.Builder
	
	// If header not printed yet, print it first
	if !p.headerPrinted {
		p.renderHeader(&sb)
		sb.WriteString("\n")
		p.headerPrinted = true
		p.lastLineCount = 0
		fmt.Print(sb.String())
		sb.Reset()
		slog.Debug("pacman header printed (TTY mode)")
	}
	
	// Always move cursor to the line after header and clear from there
	// This ensures we overwrite any previous asset lines
	sb.WriteString("\033[1G") // Move to column 1
	sb.WriteString("\033[2J") // Clear entire screen
	sb.WriteString("\033[1;1H") // Move to top-left
	
	// Reprint header
	p.renderHeader(&sb)
	sb.WriteString("\n")
	
	// Render asset lines
	p.renderAssets(&sb)
	
	fmt.Print(sb.String())
}

func (p *PacmanUI) renderHeader(sb *strings.Builder) {
	octopus := "🐙"
	tag := "🏷️"
	packageIcon := "📦"

	if p.DisableIcons {
		octopus = "R:"
		tag = "V:"
		packageIcon = "A:"
	}

	header := fmt.Sprintf("%s %s      %s %s      %s %s",
		octopus, p.Repo, tag, p.Version, packageIcon, p.Archive)
	sb.WriteString(header)
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
		GlobalPacman.renderHeader(&sb)
		sb.WriteString("\n")
		GlobalPacman.renderAssets(&sb)
		fmt.Print(sb.String())
	}
	return n, err
}
