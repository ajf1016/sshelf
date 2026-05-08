package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/ui"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import an existing ~/.ssh/config into sshelf",
	Long: `Scan your existing SSH configuration for Host blocks and key files,
then interactively migrate them into sshelf management. No existing
data is deleted without explicit confirmation.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		sshDir, err := core.DefaultSSHDir()
		if err != nil {
			return fmt.Errorf("resolve ssh dir: %w", err)
		}

		scanner := core.NewImportScanner(
			sshDir,
			app.hosts,
			app.keys,
			app.sshConfig,
		)

		imported := 0

		// ── Scan and import key files ─────────────────────────────
		keyCandidates, err := scanner.ScanKeys()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: scan keys: %v\n", err)
		}

		if len(keyCandidates) > 0 {
			fmt.Printf("Found %d key file(s) in %s not yet managed by sshelf.\n\n",
				len(keyCandidates), sshDir)

			var selectedKeys []string
			keyOptions := make([]huh.Option[string], 0, len(keyCandidates))
			for _, c := range keyCandidates {
				keyOptions = append(keyOptions, huh.NewOption(c.KeyName, c.KeyPath))
			}

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Select keys to import").
						Description("Space to toggle, Enter to confirm").
						Options(keyOptions...).
						Value(&selectedKeys),
				),
			)
			if err := form.Run(); err == nil {
				for _, srcPath := range selectedKeys {
					kInfo, err := scanner.ImportKey(srcPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "  skip %s: %v\n", filepath.Base(srcPath), err)
						continue
					}
					fmt.Printf("  %s imported key: %s\n", ui.StyleGreen.Render("✓"), kInfo.Name)
					imported++
				}
			}
		}

		// ── Scan and import host blocks ───────────────────────────
		hostCandidates, err := scanner.ScanHosts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: scan hosts: %v\n", err)
		}

		if len(hostCandidates) > 0 {
			fmt.Printf("\nFound %d host block(s) not yet managed by sshelf.\n\n",
				len(hostCandidates))

			var selectedHosts []string
			hostOptions := make([]huh.Option[string], 0, len(hostCandidates))
			for _, h := range hostCandidates {
				label := fmt.Sprintf("%s → %s", h.Alias, h.Hostname)
				hostOptions = append(hostOptions, huh.NewOption(label, h.Alias))
			}

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Select hosts to import").
						Description("Space to toggle, Enter to confirm").
						Options(hostOptions...).
						Value(&selectedHosts),
				),
			)

			if err := form.Run(); err == nil && len(selectedHosts) > 0 {
				// Build alias → ParsedHost map.
				hostMap := make(map[string]*core.ParsedHost, len(hostCandidates))
				for _, h := range hostCandidates {
					hostMap[h.Alias] = h
				}

				// Let user optionally link all selected hosts to a profile.
				profiles, _ := app.profiles.List()
				var linkProfile string
				if len(profiles) > 0 {
					profileOptions := []huh.Option[string]{huh.NewOption("(none)", "")}
					for _, p := range profiles {
						profileOptions = append(profileOptions, huh.NewOption(p.Name, p.Name))
					}
					_ = huh.NewForm(
						huh.NewGroup(
							huh.NewSelect[string]().
								Title("Link imported hosts to a profile?").
								Options(profileOptions...).
								Value(&linkProfile),
						),
					).Run()
				}

				currentProfiles, _ := app.profiles.List()
				for _, alias := range selectedHosts {
					ph, ok := hostMap[alias]
					if !ok {
						continue
					}
					if err := scanner.ImportHost(ph, linkProfile, "", currentProfiles); err != nil {
						fmt.Fprintf(os.Stderr, "  skip %s: %v\n", alias, err)
						continue
					}
					fmt.Printf("  %s imported host: %s → %s\n",
						ui.StyleGreen.Render("✓"), alias, ph.Hostname)
					imported++
				}
			}
		}

		if imported == 0 && len(keyCandidates) == 0 && len(hostCandidates) == 0 {
			fmt.Println("Nothing to import — everything in ~/.ssh/ is already managed or empty.")
			return nil
		}

		if imported > 0 {
			fmt.Printf("\n%d item(s) imported successfully.\n", imported)
			fmt.Printf("SSH config updated. Keys stored in: %s\n",
				filepath.Join(app.store.Dir(), config.KeysDir))
		}
		return nil
	},
}
