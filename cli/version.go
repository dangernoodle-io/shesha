package cli

import (
	"fmt"

	"github.com/dangernoodle-io/shesha"
	"github.com/spf13/cobra"
)

// VersionCmd builds a `version` command printing "<Name> <Version>".
func VersionCmd(info shesha.Info) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the server name and version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", info.Name, info.Version)
			return err
		},
	}
}
