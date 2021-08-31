package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.0.7"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of the KIA CLI",
	Long:  `All software has versions. This is KIA CLI's'`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
