package cmd

import (
	"context"
	"time"
)

var (
	sudoKeepaliveInterval = 60 * time.Second
)

// startSudoKeepalive spawns a background goroutine that periodically refreshes the
// sudo credentials cache using "sudo -n -v" so long-running operations (like source builds
// or large asset extractions) do not expire the sudo timestamp and hang in headless mode.
func startSudoKeepalive(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(sudoKeepaliveInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = execCommand("sudo", "-n", "-v").Run()
			}
		}
	}()
	return cancel
}
