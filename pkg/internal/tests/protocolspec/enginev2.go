package protocolspec

func getOldInputsV2[S PulumiSchema]() PulumiInput[S] {
	return PulumiInput[S]{}
}

func CreateResourceV2() {
	type PS struct{}
	type TFS struct{}
	prov := &BridgeV2[PS, TFS]{}
	userInputs := getUserInputs[PS]()

	checkedInputs := prov.Check(userInputs)
	previewOutputs := prov.CreatePreview(checkedInputs)

	displayPreviewV2(previewOutputs, PulumiOutput[PS]{}, PulumiDetailedDiffV2[PS]{})
}


func UpdateResourceWithUpgradeV2() {
	type CPS struct{}
	type CTFS struct{}
	type PPS struct{}
	type PTFS struct{}
	prov := &BridgeV2WithUpgrade[CPS, PPS, CTFS, PTFS]{}
	userInputs := getUserInputs[CPS]()
	oldState := getOldState[PPS]()
	// oldInputs := getOldInputs[PPS]() not used anymore

	checkedInputs := prov.Check(userInputs)
	upgradedState := prov.UpgradeState(oldState)

	proposedState := prov.UpdatePreview(checkedInputs, upgradedState)

	diff, detailedDiff := prov.Diff(upgradedState, proposedState)

	displayPreviewV2(proposedState, upgradedState, detailedDiff)

	if diff.HasChanges {
		prov.Update(checkedInputs, upgradedState)
	}
}

// Assumes run-program
func DeleteResourceWithUpgradeV2() {
	type CPS struct{}
	type CTFS struct{}
	type PPS struct{}
	type PTFS struct{}

	prov := &BridgeV2WithUpgrade[CPS, PPS, CTFS, PTFS]{}
	oldState := getOldState[PPS]()
	upgradedState := prov.UpgradeState(oldState)

	prov.Delete(upgradedState)
}

func RefreshWithUpgradeV2() {
	type CPS struct{}
	type CTFS struct{}
	type PPS struct{}
	type PTFS struct{}

	prov := &BridgeV2WithUpgrade[CPS, PPS, CTFS, PTFS]{}
	state := getOldState[PPS]()

	upgradedState := prov.UpgradeState(state)
	readState := prov.ReadForRefresh(upgradedState)

	promoteToPreviewOutput := func(state PulumiOutput[CPS]) PulumiPreviewOutput[CPS] {
		return PulumiPreviewOutput[CPS]{}
	}

	promotedUpgradedState := promoteToPreviewOutput(upgradedState)

	//nolint:lll
	// parameter inversion here: https://pulumi-developer-docs.readthedocs.io/latest/developer-docs/providers/implementers-guide.html#refresh
	diff, detailedDiff := prov.Diff(
		readState,
		promotedUpgradedState,
	)

	displayPreviewV2(promotedUpgradedState, readState, detailedDiff)
	if diff.HasChanges {
		// save the state
	}
}