package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPacmanUI_PhaseTransitions(t *testing.T) {
	tests := []struct {
		name              string
		initialPhase      int
		headerEaten       int
		assets            []AssetInfo
		expectedPhase     int
		expectedHeaderEat int
		expectedAsset     int
	}{
		{
			name:              "phase 0 increments headerEaten",
			initialPhase:      0,
			headerEaten:       0,
			assets:            nil,
			expectedPhase:     0,
			expectedHeaderEat: 1,
			expectedAsset:     0,
		},
		{
			name:              "phase 0 transitions to phase 1 when header complete",
			initialPhase:      0,
			headerEaten:       4,
			assets:            nil,
			expectedPhase:     1,
			expectedHeaderEat: 4,
			expectedAsset:     0,
		},
		{
			name:              "phase 1 transitions to phase 2 when assets resolved",
			initialPhase:      1,
			headerEaten:       4,
			assets:            []AssetInfo{{Name: "test-asset"}},
			expectedPhase:     2,
			expectedHeaderEat: 4,
			expectedAsset:     0,
		},
		{
			name:              "phase 2 completes assets",
			initialPhase:      2,
			headerEaten:       4,
			assets:            []AssetInfo{{Name: "test-asset", Completed: false}},
			expectedPhase:     2,
			expectedHeaderEat: 4,
			expectedAsset:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PacmanUI{
				Repo:        "test/repo",
				Version:     "v1.0.0",
				Archive:     "test.tar.gz",
				phase:       tt.initialPhase,
				headerEaten: tt.headerEaten,
				Assets:      tt.assets,
			}

			p.tick()

			if p.phase != tt.expectedPhase {
				t.Errorf("phase = %d, want %d", p.phase, tt.expectedPhase)
			}
			if p.headerEaten != tt.expectedHeaderEat {
				t.Errorf("headerEaten = %d, want %d", p.headerEaten, tt.expectedHeaderEat)
			}
			if len(tt.assets) > 0 && p.CurrentAsset != tt.expectedAsset {
				t.Errorf("CurrentAsset = %d, want %d", p.CurrentAsset, tt.expectedAsset)
			}
		})
	}
}

func TestPacmanUI_AssetCompletion(t *testing.T) {
	p := &PacmanUI{
		Repo:        "test/repo",
		Version:     "v1.0.0",
		Archive:     "test.tar.gz",
		phase:       2,
		headerEaten: 4,
		Assets: []AssetInfo{
			{Name: "asset1", Completed: false},
			{Name: "asset2", Completed: false},
		},
		CurrentAsset: 0,
		dotsEaten:    0,
	}

	// Tick 3 times to complete first asset
	for i := 0; i < 3; i++ {
		p.tick()
	}

	if !p.Assets[0].Completed {
		t.Error("first asset should be completed after 3 ticks")
	}
	if p.CurrentAsset != 1 {
		t.Errorf("CurrentAsset = %d, want 1", p.CurrentAsset)
	}

	// Tick 3 more times to complete second asset
	for i := 0; i < 3; i++ {
		p.tick()
	}

	if !p.Assets[1].Completed {
		t.Error("second asset should be completed after 3 more ticks")
	}
	if p.CurrentAsset != 2 {
		t.Errorf("CurrentAsset = %d, want 2", p.CurrentAsset)
	}
}

func TestPacmanUI_RenderNonTTY(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := &PacmanUI{
		Repo:    "test/repo",
		Version: "v1.0.0",
		Archive: "test.tar.gz",
	}

	// Render in non-TTY mode (piped output)
	p.render()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to copy output: %v", err)
	}
	os.Stdout = oldStdout

	output := buf.String()

	// Should contain header
	if !strings.Contains(output, "test/repo") {
		t.Error("output should contain repo name")
	}
	if !strings.Contains(output, "v1.0.0") {
		t.Error("output should contain version")
	}

	// Should NOT contain cursor control codes in non-TTY mode
	if strings.Contains(output, "\033[") {
		t.Error("non-TTY mode should not contain cursor control codes")
	}
}

func TestPacmanUI_HeaderPrinted(t *testing.T) {
	p := &PacmanUI{
		Repo:    "test/repo",
		Version: "v1.0.0",
		Archive: "test.tar.gz",
	}

	if p.headerPrinted {
		t.Error("headerPrinted should be false initially")
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	p.render()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to copy output: %v", err)
	}
	os.Stdout = oldStdout

	if !p.headerPrinted {
		t.Error("headerPrinted should be true after first render")
	}
}

func TestPacmanUI_AddAsset(t *testing.T) {
	p := &PacmanUI{
		Repo:    "test/repo",
		Version: "v1.0.0",
		Archive: "test.tar.gz",
	}

	if len(p.Assets) != 0 {
		t.Error("should start with no assets")
	}

	p.AddAsset("asset1", "/path/to/asset1", "", "sudo apt install asset1")

	if len(p.Assets) != 1 {
		t.Errorf("should have 1 asset, got %d", len(p.Assets))
	}
	if p.Assets[0].Name != "asset1" {
		t.Errorf("asset name = %s, want asset1", p.Assets[0].Name)
	}
	if p.Assets[0].InstallCmd != "sudo apt install asset1" {
		t.Errorf("install cmd = %s, want 'sudo apt install asset1'", p.Assets[0].InstallCmd)
	}

	p.AddAsset("asset2", "/path/to/asset2", "/symlink/path", "")

	if len(p.Assets) != 2 {
		t.Errorf("should have 2 assets, got %d", len(p.Assets))
	}
	if p.Assets[1].Symlink != "/symlink/path" {
		t.Errorf("symlink = %s, want /symlink/path", p.Assets[1].Symlink)
	}
}

func TestPacmanUI_CompleteAllAnimations(t *testing.T) {
	p := &PacmanUI{
		Repo:    "test/repo",
		Version: "v1.0.0",
		Archive: "test.tar.gz",
		Assets: []AssetInfo{
			{Name: "asset1", Completed: false},
			{Name: "asset2", Completed: false},
		},
		CurrentAsset: 0,
	}

	p.completeAllAnimations()

	if p.phase != 2 {
		t.Errorf("phase = %d, want 2", p.phase)
	}
	if p.CurrentAsset != 2 {
		t.Errorf("CurrentAsset = %d, want 2", p.CurrentAsset)
	}
	for i, asset := range p.Assets {
		if !asset.Completed {
			t.Errorf("asset %d should be completed", i)
		}
	}
}
