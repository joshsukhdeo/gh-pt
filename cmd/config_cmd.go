package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/release"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/pterm/pterm"
)

func ConfigLs() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", cfg)
	return nil
}

func ConfigGet(key string) error {
	return fmt.Errorf("not fully implemented")
}

func ConfigSet(key, val string) error {
	return fmt.Errorf("not fully implemented")
}

func ConfigRm(key string) error {
	return fmt.Errorf("not fully implemented")
}

func ConfigMenu() error {
	pterm.Info.Println("Config interactive menu not fully implemented yet.")
	return nil
}

func RunAIScan(target, aiCmd string) error {
	pterm.Info.Println("Running AI scan on", target)

	prompt := fmt.Sprintf("Analyze the GitHub repository %s for safety concerns, malicious code, suspicious recent commits, or backdoors. Report your findings concisely and explicitly state if it appears safe or compromised.", target)

	if err := runAIAgent(aiCmd, prompt, ""); err != nil {
		return fmt.Errorf("AI safety scan failed to execute: %w", err)
	}

	st, err := state.LoadState()
	if err == nil {
		if app, exists := st.Apps[target]; exists {
			app.LastAIScan = time.Now().Format(time.RFC3339)
			if err := st.Save(); err != nil {
				pterm.Warning.Println("Failed to save state after AI scan")
			}
		}
	}

	pterm.Success.Printf("Successfully completed AI scan for %s\n", target)
	return nil
}

func RunVTScan(target string) error {
	pterm.Info.Println("Running VT scan on", target)

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	apiKey := cfg.Core.VTApiKey
	if os.Getenv("VT_API_KEY") != "" {
		apiKey = os.Getenv("VT_API_KEY")
	}

	if apiKey == "" {
		return fmt.Errorf("virustotal api key is not set")
	}

	st, err := state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	app, exists := st.Apps[target]
	if !exists {
		// Let's see if target is a path
		if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
			hash, err := release.CalculateSHA256(target)
			if err != nil {
				return fmt.Errorf("failed to hash %s: %w", target, err)
			}
			return release.VerifyHashWithVirusTotal(hash, target, apiKey, true, false)
		}
		return fmt.Errorf("app %s not found in state and is not a valid file path", target)
	}

	if len(app.Rename) == 0 {
		return fmt.Errorf("no binaries mapped in state for %s", target)
	}

	for _, destName := range app.Rename {
		binPath := filepath.Join(app.TargetPath, destName)
		hash, err := release.CalculateSHA256(binPath)
		if err != nil {
			pterm.Warning.Printf("Could not hash binary %s: %v\n", binPath, err)
			continue
		}

		if err := release.VerifyHashWithVirusTotal(hash, binPath, apiKey, true, false); err != nil {
			return fmt.Errorf("virustotal scan failed for %s: %w", binPath, err)
		}
	}

	app.LastVTScan = time.Now().Format(time.RFC3339)
	if err := st.Save(); err != nil {
		return fmt.Errorf("could not update state: %w", err)
	}

	pterm.Success.Printf("Successfully scanned %s and updated state.\n", target)
	return nil
}

func SetVTKey(key string) error {
	pterm.Info.Println("Setting VT key to", key)
	return nil
}
