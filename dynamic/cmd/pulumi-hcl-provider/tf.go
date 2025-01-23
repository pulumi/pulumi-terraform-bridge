package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
)

type TFWorkspace struct {
	tfe *tfexec.Terraform
}

func newTFWorkspace(workingDir string, tfstate []byte) (*TFWorkspace, error) {

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd failed: %w", err)
	}

	d := filepath.Join(wd, ".hcl", "temp-workspace")

	execPath := "?"
	tfe, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		return nil, fmt.Errorf("error running NewTerraform: %s", err)
	}
	return nil
}

func (w *TFWorkspace) Plan(ctx context.Context) (*tfjson.Plan, error) {
	var planOutput bytes.Buffer
	ok, err := w.tfe.PlanJSON(ctx, &planOutput, tfexec.Out("tfplan"))
	if err != nil {
		return nil, fmt.Errorf("tf plan failed: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("tf plan failed: %s", planOutput.String())
	}
	plan, err := w.tfe.ShowPlanFile(ctx, "tfplan")
	if err != nil {
		return nil, fmt.Errorf("tf show -plan failed: %w", err)
	}
	return plan, nil
}

func (w *TFWorkspace) Apply(ctx context.Context) (*tfjson.State, error) {
	w.tfe.Apply(ctx, &tfexec.DirOrPlanOption{})
}
