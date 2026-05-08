package commands

// This file wires up cobra's ValidArgsFunction for commands that accept named
// entities so that shell tab-completion resolves live profile/host/key names.

import "github.com/spf13/cobra"

func init() {
	// Profile name completions.
	for _, cmd := range []*cobra.Command{
		profileSwitchCmd,
		profileShowCmd,
		profileRemoveCmd,
		profileEditCmd,
	} {
		cmd.ValidArgsFunction = completeProfileNames
	}
	profileCloneCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeProfileNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	profileRenameCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeProfileNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Key name completions.
	for _, cmd := range []*cobra.Command{
		keyRemoveCmd,
		keyRotateCmd,
		keyCopyCmd,
	} {
		cmd.ValidArgsFunction = completeKeyNames
	}

	// Host alias completions.
	for _, cmd := range []*cobra.Command{
		hostRemoveCmd,
		hostTestCmd,
		hostConnectCmd,
		hostCopyIDCmd,
		hostJumpCmd,
	} {
		cmd.ValidArgsFunction = completeHostAliases
	}
}

func completeProfileNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	app, err := getApp()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	profiles, err := app.profiles.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeKeyNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	app, err := getApp()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	keys, err := app.keys.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, k.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeHostAliases(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	app, err := getApp()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	hosts, err := app.hosts.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	aliases := make([]string, 0, len(hosts))
	for _, h := range hosts {
		aliases = append(aliases, h.Name)
	}
	return aliases, cobra.ShellCompDirectiveNoFileComp
}
