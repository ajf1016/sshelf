// Package wizard provides interactive terminal forms (powered by charmbracelet/huh)
// for the profile init, profile edit, and import flows. Each function drives
// one step of the wizard and returns the collected input to the command layer.
package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/ajf1016/sshelf/internal/core"
)

// SelectKeys presents a multiselect form for choosing which key files to
// import. Returns the paths of the selected candidates.
func SelectKeys(candidates []core.ImportCandidate) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	opts := make([]huh.Option[string], 0, len(candidates))
	for _, c := range candidates {
		opts = append(opts, huh.NewOption(c.KeyName, c.KeyPath))
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select keys to import").
				Description("Space to toggle, Enter to confirm").
				Options(opts...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("key selection cancelled: %w", err)
	}
	return selected, nil
}

// SelectHosts presents a multiselect form for choosing which SSH Host blocks
// to import. Returns the aliases of the selected candidates.
func SelectHosts(candidates []*core.ParsedHost) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	opts := make([]huh.Option[string], 0, len(candidates))
	for _, h := range candidates {
		label := fmt.Sprintf("%s → %s", h.Alias, h.Hostname)
		opts = append(opts, huh.NewOption(label, h.Alias))
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select hosts to import").
				Description("Space to toggle, Enter to confirm").
				Options(opts...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("host selection cancelled: %w", err)
	}
	return selected, nil
}

// SelectProfileLink presents a single-select form for optionally linking
// imported hosts to an existing profile. Returns the chosen profile name, or
// an empty string if the user picks "(none)".
func SelectProfileLink(profiles []*core.Profile) (string, error) {
	if len(profiles) == 0 {
		return "", nil
	}

	opts := []huh.Option[string]{huh.NewOption("(none)", "")}
	for _, p := range profiles {
		opts = append(opts, huh.NewOption(p.Name, p.Name))
	}

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Link imported hosts to a profile?").
				Options(opts...).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("profile link cancelled: %w", err)
	}
	return choice, nil
}
