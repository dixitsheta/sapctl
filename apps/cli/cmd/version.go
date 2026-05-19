package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionInfo is the JSON payload emitted by `sapctl version --json`.
type versionInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the sapctl version",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := versionInfo{
				Version:   version,
				GoVersion: runtime.Version(),
				OS:        runtime.GOOS,
				Arch:      runtime.GOARCH,
			}
			if globalFlags.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				if globalFlags.Compact {
					return enc.Encode(info)
				}
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
			return err
		},
	}
}
