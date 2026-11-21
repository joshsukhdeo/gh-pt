package cmd

import (
	"context"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartSudoKeepalive(t *testing.T) {
	origExecCommand := execCommand
	origInterval := sudoKeepaliveInterval
	defer func() {
		execCommand = origExecCommand
		sudoKeepaliveInterval = origInterval
	}()

	var callCount atomic.Int32
	var lastCmd string
	var lastArgs []string

	execCommand = func(name string, arg ...string) *exec.Cmd {
		callCount.Add(1)
		lastCmd = name
		lastArgs = append([]string(nil), arg...)
		return exec.Command("true")
	}

	sudoKeepaliveInterval = 15 * time.Millisecond

	ctx := context.Background()
	cancel := startSudoKeepalive(ctx)

	// Wait long enough for at least 2 ticks
	time.Sleep(45 * time.Millisecond)

	cancel()
	// Allow in-flight goroutine iteration to cleanly exit
	time.Sleep(10 * time.Millisecond)

	countAfterCancel := callCount.Load()
	assert.GreaterOrEqual(t, countAfterCancel, int32(2), "should have ticked at least twice")
	assert.Equal(t, "sudo", lastCmd)
	assert.Equal(t, []string{"-n", "-v"}, lastArgs)

	// Sleep more to verify ticker has stopped
	time.Sleep(35 * time.Millisecond)
	countLater := callCount.Load()
	assert.Equal(t, countAfterCancel, countLater, "goroutine must stop after context cancellation")
}
