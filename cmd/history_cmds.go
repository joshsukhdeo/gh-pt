package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/joshsukhdeo/gh-pt/ui"
	"github.com/pterm/pterm"
	"golang.org/x/term"
)

// RunHistory handles viewing and pruning the update history database.
func RunHistory(cli *params.HistoryCmd) error {
	ApplyDisplayPreferences(cli.NoColor, cli.NoEmojis)

	days := cli.Days
	if days <= 0 {
		days = 30
	}
	retention := time.Duration(days) * 24 * time.Hour

	if cli.Clean {
		pruned, err := state.PruneHistory(retention)
		if err != nil {
			return fmt.Errorf("failed to prune history: %w", err)
		}
		pterm.Success.Printf("Pruned %d history record(s) and associated backup files older than %d days.\n", pruned, days)
		if cli.Repository == "" {
			return nil
		}
	}

	records, err := state.GetHistory(cli.Repository, retention)
	if err != nil {
		return fmt.Errorf("failed to fetch history: %w", err)
	}

	if len(records) == 0 {
		msg := fmt.Sprintf("No update history found within the last %d days.", days)
		if cli.Repository != "" {
			msg = fmt.Sprintf("No update history found for %s within the last %d days.", cli.Repository, days)
		}
		pterm.Info.Println(msg)
		return nil
	}

	if cli.Format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}

	// Format as Table
	tableData := pterm.TableData{
		{"TIMESTAMP", "ACTION", "REPOSITORY", "FROM", "TO", "STATUS"},
	}

	for _, rec := range records {
		ts := rec.Timestamp.Local().Format("2006-01-02 15:04:05")
		from := rec.PreviousVersion
		if from == "" {
			from = "-"
		}
		to := rec.NewVersion
		if to == "" {
			to = "-"
		}
		status := "success"
		if !rec.Success {
			status = "failed"
		}
		tableData = append(tableData, []string{
			ts,
			rec.Action,
			rec.Repository,
			from,
			to,
			status,
		})
	}

	return pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
}

// RunRollback restores an installed application to its previous version.
func RunRollback(cli *params.RollbackCmd) error {
	ApplyDisplayPreferences(cli.NoColor, cli.NoEmojis)

	if cli.Repository == "" {
		return fmt.Errorf("repository argument is required for rollback")
	}

	// First execute or simulate the rollback
	res, err := state.RollbackApp(cli.Repository, cli.To, cli.DryRun)
	if err != nil {
		return err
	}

	if cli.DryRun {
		pterm.Info.Printf("[dry-run] Rollback plan for %s:\n", cli.Repository)
		fmt.Printf("  • Current version:  %s\n", res.FromVersion)
		fmt.Printf("  • Target version:   %s\n", res.ToVersion)
		if res.NeedsReinstall {
			pterm.Warning.Println("  • Local backup binaries not found; rolling back state will require running:")
			fmt.Printf("    gh-pt %s -v %s -f\n", cli.Repository, res.ToVersion)
		} else {
			fmt.Printf("  • Backup files to restore (%d file(s)):\n", len(res.RestoredFiles))
			for _, f := range res.RestoredFiles {
				fmt.Printf("    - %s\n", f)
			}
		}
		return nil
	}

	// If interactive and not forced, prompt for confirmation
	isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if !cli.Force && isInteractive {
		confirmMsg := fmt.Sprintf("Roll back %s from %s to %s?", cli.Repository, res.FromVersion, res.ToVersion)
		var confirmed bool
		err := huh.NewConfirm().
			Title(confirmMsg).
			Value(&confirmed).
			Run()
		if err != nil || !confirmed {
			pterm.Warning.Println("Rollback aborted by user.")
			return nil
		}
	}

	successMsg := fmt.Sprintf("✨ Successfully rolled back %s from %s to %s!", cli.Repository, res.FromVersion, res.ToVersion)
	if cli.NoEmojis {
		successMsg = ui.StripEmojis(successMsg)
	}
	pterm.Success.Println(successMsg)

	if res.NeedsReinstall {
		pterm.Warning.Printf("Local binary backups were missing for %s. State version was updated to %s.\n", cli.Repository, res.ToVersion)
		pterm.Info.Printf("To complete reinstalling binaries, run:\n  gh-pt %s -v %s -f\n", cli.Repository, res.ToVersion)
	} else if len(res.RestoredFiles) > 0 {
		fmt.Printf("Restored %d binary file(s):\n", len(res.RestoredFiles))
		for _, f := range res.RestoredFiles {
			fmt.Printf("  • %s\n", f)
		}
	}

	return nil
}
