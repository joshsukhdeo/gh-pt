package cmd

import (
	"fmt"
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
	pterm.Info.Println("Config interactive menu not fully implemented yet.")
	return nil
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
