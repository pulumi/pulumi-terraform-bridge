package protocolspec

// CPS stands for Current Pulumi Schema
// PPS stands for Previous Pulumi Schema
// CTFS stands for Current TF Schema
// PTFS stands for Previous TF Schema
type Bridge[CPS PulumiSchema, PPS PulumiSchema, CTFS TFSchema, PTFS TFSchema] struct {
	TFProvider TFProviderWithUpgradeState[CTFS, PTFS]
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) getCurrentBridge() Bridge[CPS, CPS, CTFS, CTFS] {
	return Bridge[CPS, CPS, CTFS, CTFS]{
		TFProvider: b.TFProvider.GetCurrentProvider(),
	}
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) tfStateToPulumiOutput(TFState[CTFS]) PulumiOutput[CPS] {
	return PulumiOutput[CPS]{}
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) tfPlannedStateToPulumiPreviewOutput(
	TFPlannedState[CTFS],
) PulumiPreviewOutput[CPS] {
	return PulumiPreviewOutput[CPS]{}
}

// This was recently replaced by tfRawStateFromPulumiOutput
func (b *Bridge[CPS, PPS, CTFS, PTFS]) pulumiOutputToTFState(PulumiOutput[CPS]) TFState[CTFS] {
	return TFState[CTFS]{}
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) pulumiCheckedInputToTFConfig(PulumiCheckedInput[CPS]) TFConfig[CTFS] {
	return TFConfig[CTFS]{}
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) tfRawStateFromPulumiOutput(PulumiOutput[PPS]) TFState[PTFS] {
	return TFState[PTFS]{}
}

// extractInputsFromOutputs is not precise and has some assumptions and approximations
// Especially when there are no past inputs
func (b *Bridge[CPS, PPS, CTFS, PTFS]) extractInputsFromOutputs(
	oldInputs PulumiCheckedInput[PPS],
	outputs PulumiOutput[CPS],
) PulumiInput[CPS] {
	return PulumiInput[CPS]{}
}

var _ PulumiProvider[PulumiSchema, PulumiSchema] = &Bridge[PulumiSchema, PulumiSchema, TFSchema, TFSchema]{}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) Check(
	news PulumiInput[CPS], olds PulumiCheckedInput[PPS],
) PulumiCheckedInput[CPS] {
	return PulumiCheckedInput[CPS]{}
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) CreatePreview(checkedInputs PulumiCheckedInput[CPS]) PulumiPreviewOutput[CPS] {
	tfConfig := b.pulumiCheckedInputToTFConfig(checkedInputs)
	emptyState := TFState[CTFS]{}
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, emptyState)
	return b.tfPlannedStateToPulumiPreviewOutput(tfPlannedState)
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) Create(checkedInputs PulumiCheckedInput[CPS]) PulumiOutput[CPS] {
	tfConfig := b.pulumiCheckedInputToTFConfig(checkedInputs)
	emptyState := TFState[CTFS]{}
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, emptyState)
	tfState := b.TFProvider.ApplyResourceChange(tfConfig, emptyState, tfPlannedState)
	return b.tfStateToPulumiOutput(tfState)
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) UpdatePreview(
	checkedInputs PulumiCheckedInput[CPS],
	state PulumiOutput[PPS],
	// oldInputs is unused
	_oldInputs PulumiCheckedInput[PPS],
) PulumiPreviewOutput[CPS] {
	tfConfig := b.pulumiCheckedInputToTFConfig(checkedInputs)
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfUpgradedState := b.TFProvider.UpgradeState(tfState)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfUpgradedState)
	return b.tfPlannedStateToPulumiPreviewOutput(tfPlannedState)
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) Update(
	checkedInputs PulumiCheckedInput[CPS],
	state PulumiOutput[PPS],
	// oldInputs is unused
	_oldInputs PulumiCheckedInput[PPS],
) PulumiOutput[CPS] {
	tfConfig := b.pulumiCheckedInputToTFConfig(checkedInputs)
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfUpgradedState := b.TFProvider.UpgradeState(tfState)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfUpgradedState)
	tfNewState := b.TFProvider.ApplyResourceChange(tfConfig, tfUpgradedState, tfPlannedState)
	return b.tfStateToPulumiOutput(tfNewState)
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) DeletePreview(state PulumiOutput[PPS]) {
	tfState := b.tfRawStateFromPulumiOutput(state)
	upgradedState := b.TFProvider.UpgradeState(tfState)
	emptyConfig := TFConfig[CTFS]{}
	b.TFProvider.PlanResourceChange(emptyConfig, upgradedState)
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) Delete(state PulumiOutput[PPS]) {
	tfState := b.tfRawStateFromPulumiOutput(state)
	upgradedState := b.TFProvider.UpgradeState(tfState)
	emptyConfig := TFConfig[CTFS]{}
	b.TFProvider.ApplyResourceChange(emptyConfig, upgradedState, TFPlannedState[CTFS]{})
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) Diff(
	oldState PulumiOutput[PPS],
	oldInputs PulumiCheckedInput[PPS],
	newCheckedInputs PulumiCheckedInput[CPS],
) (PulumiDiff[CPS], PulumiDetailedDiff[CPS, PPS]) {
	tfState := b.tfRawStateFromPulumiOutput(oldState)
	tfUpgradedState := b.TFProvider.UpgradeState(tfState)
	tfConfig := b.pulumiCheckedInputToTFConfig(newCheckedInputs)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfUpgradedState)
	plannedState := b.tfPlannedStateToPulumiPreviewOutput(tfPlannedState)

	tfDiff := b.TFProvider.Diff(tfUpgradedState, tfPlannedState)

	translateTFDiffToPulumiDiff := func(TFDiff[CTFS]) PulumiDiff[CPS] {
		return PulumiDiff[CPS]{}
	}

	producePulumiDetailedDiff := func(
		_ TFDiff[CTFS],
		oldState PulumiOutput[PPS],
		newCheckedInputs PulumiCheckedInput[CPS],
		_ PulumiPreviewOutput[CPS],
	) PulumiDetailedDiff[CPS, PPS] {
		// This is not easy as we first need to diff PulumiOutput[PPS] against PulumiPreviewOutput[CPS]
		// and then we need to map this back to the PulumiCheckedInput[CPS]
		// PulumiPreviewOutput and PulumiCheckedInput are not directly comparable
		return PulumiDetailedDiff[CPS, PPS]{
			oldState:         oldState,
			newCheckedInputs: newCheckedInputs,
		}
	}

	pulumiDiff := translateTFDiffToPulumiDiff(tfDiff)
	pulumiDetailedDiff := producePulumiDetailedDiff(tfDiff, oldState, newCheckedInputs, plannedState)

	return pulumiDiff, pulumiDetailedDiff
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) ReadForImport(id string) (PulumiOutput[CPS], PulumiInput[CPS]) {
	tfState := b.TFProvider.Importer(id)
	state := b.tfStateToPulumiOutput(tfState)
	inputs := b.extractInputsFromOutputs(PulumiCheckedInput[PPS]{}, state)
	return state, inputs
}

func (b *Bridge[CPS, PPS, CTFS, PTFS]) ReadForRefresh(
	inputs PulumiCheckedInput[PPS], state PulumiOutput[PPS],
) (PulumiInput[CPS], PulumiOutput[CPS]) {
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfUpgradedState := b.TFProvider.UpgradeState(tfState)

	tfNewState := b.TFProvider.ReadResource(tfUpgradedState)

	newPulumiState := b.tfStateToPulumiOutput(tfNewState)

	newPulumiInputs := b.extractInputsFromOutputs(inputs, newPulumiState)
	return newPulumiInputs, newPulumiState
}
