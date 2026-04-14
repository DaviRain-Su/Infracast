package commands

import (
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root command
func NewRootCommand(version, commit, buildTime string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "infracast",
		Short: "Infracast - Code-First infrastructure automation for Chinese clouds",
		Long: `Infracast is a Code-First infrastructure automation platform 
based on Encore framework, designed for Chinese cloud providers 
(Alicloud, Huawei Cloud, Tencent Cloud, Volcengine).

It enables developers to deploy applications to Chinese clouds 
with minimal infrastructure configuration.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path (default: infracast.yaml)")
	rootCmd.PersistentFlags().StringP("env", "e", "dev", "target environment")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(newVersionCommand(version, commit, buildTime))
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newRunCommand())
	rootCmd.AddCommand(newDeployCommand())
	rootCmd.AddCommand(newEnvCommand())
	rootCmd.AddCommand(newProvisionCommand())
	rootCmd.AddCommand(newDestroyCommand())
	rootCmd.AddCommand(newStatusCommand())

	return rootCmd
}
