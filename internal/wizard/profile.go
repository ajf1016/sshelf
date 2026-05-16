// Package wizard provides interactive terminal forms (powered by charmbracelet/huh)
// for the profile init, profile edit, and import flows. Each function drives
// one step of the wizard and returns the collected input to the command layer.
package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
)

// ProfileInitInput holds the raw form data collected during profile creation.
// The command layer is responsible for calling core functions with these fields.
type ProfileInitInput struct {
	Name     string
	Type     string
	Platform string
	Email    string
	Username string
	KeyName  string
	KeyType  string
}

// RunProfileInit runs the interactive profile creation wizard in two passes and
// returns the collected input. Returns an error if the user cancels.
//
// Pass 1 collects identity fields (name, type, platform, email, username).
// Pass 2 collects key config, pre-filling the key name with <name>_ed25519 so
// the user can accept the default or edit it.
func RunProfileInit() (*ProfileInitInput, error) {
	var in ProfileInitInput

	// Pass 1 — identity.
	pass1 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Profile name").
				Description("Short identifier, e.g. work or personal").
				Validate(huh.ValidateNotEmpty()).
				Value(&in.Name),

			huh.NewSelect[string]().
				Title("Profile type").
				Options(
					huh.NewOption("git (GitHub / GitLab / Bitbucket)", config.ProfileTypeGit),
					huh.NewOption("server (SSH into remote machines)", config.ProfileTypeServer),
					huh.NewOption("client (custom / other)", config.ProfileTypeClient),
				).
				Value(&in.Type),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Platform").
				Options(
					huh.NewOption("GitHub", config.PlatformGitHub),
					huh.NewOption("GitLab", config.PlatformGitLab),
					huh.NewOption("Bitbucket", config.PlatformBitbucket),
					huh.NewOption("Other", config.PlatformOther),
				).
				Value(&in.Platform),

			huh.NewInput().
				Title("Email").
				Validate(huh.ValidateNotEmpty()).
				Value(&in.Email),

			huh.NewInput().
				Title("Username / git user").
				Validate(huh.ValidateNotEmpty()).
				Value(&in.Username),
		),
	)
	if err := pass1.Run(); err != nil {
		return nil, fmt.Errorf("wizard cancelled: %w", err)
	}

	// Derive a sensible default key name from the profile name so the user
	// sees a pre-filled value they can accept with Enter or edit freely.
	in.KeyName = fmt.Sprintf("%s_ed25519", in.Name)

	// Pass 2 — key config, with the derived default already in place.
	pass2 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Key name").
				Description("Base filename for the key pair (edit or press Enter to accept)").
				Validate(huh.ValidateNotEmpty()).
				Value(&in.KeyName),

			huh.NewSelect[string]().
				Title("Key type").
				Options(
					huh.NewOption("ed25519 (recommended)", config.KeyTypeED25519),
					huh.NewOption("rsa 4096", config.KeyTypeRSA),
				).
				Value(&in.KeyType),
		),
	)
	if err := pass2.Run(); err != nil {
		return nil, fmt.Errorf("wizard cancelled: %w", err)
	}

	return &in, nil
}

// RunProfileEdit runs the interactive profile edit form pre-populated with the
// current profile values. Returns a shallow copy of p with the updated fields.
func RunProfileEdit(p *core.Profile) (*core.Profile, error) {
	updated := *p

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Email").Value(&updated.Email),
			huh.NewInput().Title("Username").Value(&updated.Username),
			huh.NewInput().Title("Key name").Value(&updated.KeyName),
			huh.NewInput().Title("Platform").Value(&updated.Platform),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("edit cancelled: %w", err)
	}
	return &updated, nil
}
