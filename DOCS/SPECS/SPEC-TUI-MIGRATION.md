# Spec: Bubble Tea v2 TUI Migration

## Objective
Modernize the `gh-pt` terminal user interface by migrating the custom pacman progress bar and interactive prompts to a unified `charm.land/bubbletea/v2` architecture. This includes implementing `harmonica` spring physics for smooth progress interpolation and `lipgloss` for centralized theming, replacing legacy `pterm` components.

## Tech Stack
- **Go**: >= 1.21
- **TUI Framework**: `charm.land/bubbletea/v2`
- **Styling**: `github.com/charmbracelet/lipgloss/v2`
- **Physics**: `github.com/charmbracelet/harmonica`

## Commands
- **Test UI package**: `go test -v ./ui/...`
- **Run CLI Integration**: `go build -o gh-pt . && ./gh-pt install <repo>`
- **Lint**: `golangci-lint run ./ui/...`

## Project Structure
- `ui/pacman.go` → The core Bubble Tea v2 model implementation.
- `ui/theme.go` → Centralized Lipgloss style definitions.
- `ui/pacman_test.go` → Headless model state testing.

## Code Style
```go
// pacmanModel implements tea.Model and the ProgressBar interface.
type pacmanModel struct {
	progress float64
	spring   harmonica.Spring
	width    int
	styles   *theme.Styles
}

func (m pacmanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		// Interpolate progress smoothly rather than jumping
		m.progress = m.spring.Update(msg.value)
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}
```

## Testing Strategy
- **Framework**: Standard `testing` package with `stretchr/testify/assert`.
- **Location**: `ui/pacman_test.go`
- **Coverage**: Focus on testing the `tea.Model.Update` state transitions headlessly (sending `tea.Msg` and asserting the resulting struct state) rather than scraping stdout.
- **Environment**: Must explicitly test TTY detection logic to ensure CI pipelines default to headless logging without hanging.

## Boundaries
- **Always**: Test UI gracefully failing back to standard logging in non-TTY environments.
- **Always**: Ensure `WaitForAnimation()` blocks the main CLI thread until the tea program exits gracefully.
- **Ask first**: Before ripping out remaining `pterm` prompts (like the multiselect) outside of the progress bar domain.
- **Never**: Introduce blocking `time.Sleep` calls in the UI thread; use `tea.Tick` or `harmonica` updates.

## Success Criteria
1. `gh-pt install` displays a smooth, non-teleporting pacman animation driven by Bubble Tea v2.
2. Running the same command with `TERM=dumb` or `> /dev/null` gracefully degrades without panic or infinite hanging.
3. `ui/pacman_test.go` executes `Update()` state transitions successfully without a real terminal attached.
