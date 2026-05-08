package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/ui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check SSH setup health and optionally auto-fix issues",
	Long: `Run a suite of checks against your SSH configuration, key permissions,
agent state, and git identity alignment.

Each check reports PASS, WARN, or FAIL. Use --fix to automatically
resolve all fixable issues (permissions, agent restart, missing dirs).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		fixMode, _ := cmd.Flags().GetBool("fix")
		checkName, _ := cmd.Flags().GetString("check")

		runner := app.doctor()

		var results []*core.CheckResult
		if checkName != "" {
			result, err := runner.Run(checkName)
			if err != nil {
				return err
			}
			results = []*core.CheckResult{result}
		} else {
			results = runner.RunAll()
		}

		// Print results table.
		headers := []string{"CHECK", "STATUS", "MESSAGE"}
		rows := make([][]string, 0, len(results))
		for _, r := range results {
			rows = append(rows, []string{
				r.Name,
				ui.CheckBadge(r.Status.String()),
				r.Message,
			})
		}
		fmt.Print(ui.Table(headers, rows))

		if !fixMode {
			return summariseResults(results)
		}

		// Auto-fix mode.
		fixed := 0
		var fixErrs []string
		for _, r := range results {
			if r.Fixable && r.Fix != nil && r.Status != core.CheckPass {
				if err := r.Fix(); err != nil {
					fixErrs = append(fixErrs, fmt.Sprintf("  %s: %v", r.Name, err))
				} else {
					fmt.Printf("  %s fixed: %s\n", ui.StyleGreen.Render("✓"), r.Name)
					fixed++
				}
			}
		}
		if fixed > 0 {
			fmt.Printf("\n%d issue(s) fixed.\n", fixed)
		}
		if len(fixErrs) > 0 {
			fmt.Fprintln(os.Stderr, "\nFix errors:")
			for _, e := range fixErrs {
				fmt.Fprintln(os.Stderr, e)
			}
			return fmt.Errorf("%d fix(es) failed", len(fixErrs))
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "auto-fix all fixable issues")
	doctorCmd.Flags().String("check", "", "run a single named check (config-dir, permissions, agent, active-profile, key-age, orphaned-keys)")
}

func summariseResults(results []*core.CheckResult) error {
	warns, fails := 0, 0
	for _, r := range results {
		switch r.Status {
		case core.CheckWarn:
			warns++
		case core.CheckFail:
			fails++
		}
	}
	if warns+fails == 0 {
		fmt.Println("\n" + ui.StyleGreen.Render("All checks passed."))
		return nil
	}
	summary := fmt.Sprintf("\n%d warning(s), %d failure(s).", warns, fails)
	if fails > 0 {
		fmt.Fprintln(os.Stderr, ui.StyleRed.Render(summary))
		fmt.Fprintln(os.Stderr, "Run `sshelf doctor --fix` to resolve fixable issues.")
		return fmt.Errorf("doctor found failures")
	}
	fmt.Println(ui.StyleYellow.Render(summary))
	fmt.Println("Run `sshelf doctor --fix` to resolve fixable issues.")
	return nil
}
