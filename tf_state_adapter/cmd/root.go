package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pulumi-terraform-state-conversion",
	Short: "Convert Terraform state files to Pulumi state format",
	Long: `A CLI tool to convert Terraform state files to Pulumi state format.
This tool helps migrate infrastructure state from Terraform to Pulumi.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
}
