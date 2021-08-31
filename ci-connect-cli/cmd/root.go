package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"strings"
)

var rootCmd = &cobra.Command{
	Use:   "ci-connect-cli",
	Short: "Keptn CI Connect CLI",
	Long: `Keptn CI Connect CLI copies configuration files for keptn services to the
keptn configuration repository and enriches them with metadata.`,
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(func() {
		viper.AutomaticEnv()
		replacer := strings.NewReplacer("-", "_")
		viper.SetEnvKeyReplacer(replacer)
		postInitCommands(rootCmd.Commands())
	})
}

func postInitCommands(commands []*cobra.Command) {
	for _, cmd := range commands {
		presetRequiredFlags(cmd)
		if cmd.HasSubCommands() {
			postInitCommands(cmd.Commands())
		}
	}
}

func presetRequiredFlags(cmd *cobra.Command) {
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		log.Fatal("could not bind flags", err)
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			err := cmd.Flags().Set(f.Name, viper.GetString(f.Name))
			if err != nil {
				fmt.Println("Could not set flags", err)
			}
		}
	})
}

func prepareGitRepoCmd(repository *gitRepositoryConfig, cmd *cobra.Command) {
	repository.remoteURI = cmd.Flags().StringP("git-repo", "r", "", "The keptn git repository uri for the service")
	repository.user = cmd.Flags().StringP("git-user", "u", "", "The git user that has access to the specified git-repo")
	repository.token = cmd.Flags().StringP("git-token", "t", "", "The git token that will be used by the git-user to access the git-repo")

	err := cmd.MarkFlagRequired("git-repo")
	if err != nil {
		fmt.Println("Could not mark field required", err)
	}

	err = cmd.MarkFlagRequired("git-user")
	if err != nil {
		fmt.Println("Could not mark field required", err)
	}

	err = cmd.MarkFlagRequired("git-token")
	if err != nil {
		fmt.Println("Could not mark field required", err)
	}

}
