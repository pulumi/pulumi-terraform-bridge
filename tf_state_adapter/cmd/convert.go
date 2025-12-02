package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/v3/tf_state_adapter/internal/adapter"

	"github.com/spf13/cobra"
)

var (
	inputFile   string
	outputFile  string
	stackFolder string
)

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert Terraform state to Pulumi state",
	Long: `Convert a Terraform state file to Pulumi state format.

Example:
  pulumi-terraform-state-conversion convert --input terraform.tfstate --output pulumi.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Converting Terraform state from: %s\n", inputFile)
		fmt.Printf("Output will be written to: %s\n", outputFile)

		data, err := adapter.Convert(inputFile, stackFolder)
		if err != nil {
			return fmt.Errorf("failed to convert Terraform state: %w", err)
		}

		if outputFile != "" {
			bytes, err := json.Marshal(data)
			if err != nil {
				return fmt.Errorf("failed to marshal Pulumi state: %w", err)
			}
			err = os.WriteFile(outputFile, bytes, 0o600)
			if err != nil {
				return fmt.Errorf("failed to write Pulumi state: %w", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(convertCmd)

	convertCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input Terraform state file (required)")
	convertCmd.Flags().StringVarP(&stackFolder, "stack-folder", "s", "", "Stack folder for Pulumi state")
	convertCmd.Flags().StringVarP(&outputFile, "output-file", "f", "", "Output Pulumi state file")

	convertCmd.MarkFlagRequired("input")
	convertCmd.MarkFlagRequired("stack-folder")
	convertCmd.MarkFlagRequired("output-file")
}
