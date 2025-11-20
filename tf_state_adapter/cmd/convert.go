package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/tf_state_adapter/internal/adapter"

	"github.com/spf13/cobra"
)

var (
	inputFile  string
	outputFile string
)

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert Terraform state to Pulumi state",
	Long: `Convert a Terraform state file to Pulumi state format.

Example:
  pulumi-terraform-state-conversion convert --input terraform.tfstate --output pulumi.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if inputFile == "" {
			return fmt.Errorf("input file is required")
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required")
		}

		fmt.Printf("Converting Terraform state from: %s\n", inputFile)
		fmt.Printf("Output will be written to: %s\n", outputFile)

		return adapter.Convert(inputFile, outputFile)
	},
}

func init() {
	rootCmd.AddCommand(convertCmd)

	convertCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input Terraform state file (required)")
	convertCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output Pulumi state file (required)")

	convertCmd.MarkFlagRequired("input")
	convertCmd.MarkFlagRequired("output")
}
