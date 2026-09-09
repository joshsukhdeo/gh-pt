package cmd

import (
	"fmt"
	"strings"

	"github.com/joshsukhdeo/gh-pt/config"
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
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	for {
		category, _ := pterm.DefaultInteractiveSelect.
			WithOptions([]string{"Paths", "AI", "Core", "Exit"}).
			WithDefaultText("Select Configuration Category").
			Show()

		if category == "Exit" {
			break
		}

		handleCategoryMenu(cfg, category)

		if err := config.SaveConfig(cfg); err != nil {
			pterm.Error.Printf("Failed to save config: %v\n", err)
		} else {
			pterm.Success.Println("Configuration saved successfully.")
		}
	}

	return nil
}

func handleCategoryMenu(cfg *config.Config, category string) {
	for {
		var options []string
		var selected string

		switch category {
		case "Paths":
			options = []string{
				fmt.Sprintf("InstallPath: %s", cfg.Paths.InstallPath),
				fmt.Sprintf("GlobalPath: %s", cfg.Paths.GlobalPath),
				fmt.Sprintf("ClonePath: %s", cfg.Paths.ClonePath),
				fmt.Sprintf("ForkPath: %s", cfg.Paths.ForkPath),
				"Back",
			}
		case "AI":
			options = []string{
				fmt.Sprintf("AICmd: %s", cfg.AI.AICmd),
				fmt.Sprintf("AIInteractiveCmd: %s", cfg.AI.AIInteractiveCmd),
				"Back",
			}
		case "Core":
			options = []string{
				fmt.Sprintf("InstallTypes: %s", cfg.Core.InstallTypes),
				fmt.Sprintf("AddDeps: %t", cfg.Core.AddDeps),
				fmt.Sprintf("NoDeps: %t", cfg.Core.NoDeps),
				fmt.Sprintf("DisablePrompts: %t", cfg.Core.DisablePrompts),
				fmt.Sprintf("NoSaveState: %t", cfg.Core.NoSaveState),
				fmt.Sprintf("Wine: %s", cfg.Core.Wine),
				fmt.Sprintf("NativeExtract: %t", cfg.Core.NativeExtract),
				fmt.Sprintf("KeepSuffixes: %t", cfg.Core.KeepSuffixes),
				fmt.Sprintf("VTApiKey: %s", cfg.Core.VTApiKey),
				fmt.Sprintf("AllowPrerelease: %t", cfg.Core.AllowPrerelease),
				fmt.Sprintf("DisableIcons: %t", cfg.Core.DisableIcons),
				fmt.Sprintf("LogToFile: %t", cfg.Core.LogToFile),
				"Back",
			}
		}

		selected, _ = pterm.DefaultInteractiveSelect.
			WithOptions(options).
			WithDefaultText(fmt.Sprintf("Select %s Field to Edit", category)).
			Show()

		if selected == "Back" {
			break
		}

		editField(cfg, category, selected)
	}
}

func editField(cfg *config.Config, category, selected string) {
	// Parse the field name from the selected option
	parts := strings.Split(selected, ":")
	if len(parts) == 0 {
		return
	}
	fieldName := strings.TrimSpace(parts[0])

	switch category {
	case "Paths":
		switch fieldName {
		case "InstallPath":
			cfg.Paths.InstallPath, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("InstallPath").WithDefaultValue(cfg.Paths.InstallPath).Show()
		case "GlobalPath":
			cfg.Paths.GlobalPath, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("GlobalPath").WithDefaultValue(cfg.Paths.GlobalPath).Show()
		case "ClonePath":
			cfg.Paths.ClonePath, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("ClonePath").WithDefaultValue(cfg.Paths.ClonePath).Show()
		case "ForkPath":
			cfg.Paths.ForkPath, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("ForkPath").WithDefaultValue(cfg.Paths.ForkPath).Show()
		}
	case "AI":
		switch fieldName {
		case "AICmd":
			cfg.AI.AICmd, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("AICmd").WithDefaultValue(cfg.AI.AICmd).Show()
		case "AIInteractiveCmd":
			cfg.AI.AIInteractiveCmd, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("AIInteractiveCmd").WithDefaultValue(cfg.AI.AIInteractiveCmd).Show()
		}
	case "Core":
		switch fieldName {
		case "InstallTypes":
			cfg.Core.InstallTypes, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("InstallTypes").WithDefaultValue(cfg.Core.InstallTypes).Show()
		case "Wine":
			cfg.Core.Wine, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Wine").WithDefaultValue(cfg.Core.Wine).Show()
		case "VTApiKey":
			cfg.Core.VTApiKey, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("VTApiKey").WithDefaultValue(cfg.Core.VTApiKey).Show()
		case "AddDeps":
			cfg.Core.AddDeps, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("AddDeps").WithDefaultValue(cfg.Core.AddDeps).Show()
		case "NoDeps":
			cfg.Core.NoDeps, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("NoDeps").WithDefaultValue(cfg.Core.NoDeps).Show()
		case "DisablePrompts":
			cfg.Core.DisablePrompts, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("DisablePrompts").WithDefaultValue(cfg.Core.DisablePrompts).Show()
		case "NoSaveState":
			cfg.Core.NoSaveState, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("NoSaveState").WithDefaultValue(cfg.Core.NoSaveState).Show()
		case "NativeExtract":
			cfg.Core.NativeExtract, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("NativeExtract").WithDefaultValue(cfg.Core.NativeExtract).Show()
		case "KeepSuffixes":
			cfg.Core.KeepSuffixes, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("KeepSuffixes").WithDefaultValue(cfg.Core.KeepSuffixes).Show()
		case "AllowPrerelease":
			cfg.Core.AllowPrerelease, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("AllowPrerelease").WithDefaultValue(cfg.Core.AllowPrerelease).Show()
		case "DisableIcons":
			cfg.Core.DisableIcons, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("DisableIcons").WithDefaultValue(cfg.Core.DisableIcons).Show()
		case "LogToFile":
			cfg.Core.LogToFile, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("LogToFile").WithDefaultValue(cfg.Core.LogToFile).Show()
		}
	}
}

func RunAIScan(target, aiCmd string) error {
	pterm.Info.Println("Running AI scan on", target)
	return nil
}

func RunVTScan(target string) error {
	pterm.Info.Println("Running VT scan on", target)
	return nil
}

func SetVTKey(key string) error {
	pterm.Info.Println("Setting VT key to", key)
	return nil
}
