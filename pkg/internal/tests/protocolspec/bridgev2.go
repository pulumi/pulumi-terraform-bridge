package protocolspec

type BridgeV2[CPS PulumiSchema, CTFS TFSchema] struct {
	TFProvider TFProvider[CTFS]
}

var _ PulumiProviderV2[PulumiSchema] = &BridgeV2[PulumiSchema, TFSchema]{}

func (b *BridgeV2[CPS, CTFS]) tfStateToPulumiOutput(TFState[CTFS]) PulumiOutput[CPS] {
	return PulumiOutput[CPS]{}
}

func (b *BridgeV2[CPS, CTFS]) tfPlannedStateToPulumiPreviewOutput(
	TFPlannedState[CTFS],
) PulumiPreviewOutput[CPS] {
	return PulumiPreviewOutput[CPS]{}
}

// This is new.
func (b *BridgeV2[CPS, CTFS]) pulumiPreviewOutputToTFPlannedState(PulumiPreviewOutput[CPS]) TFPlannedState[CTFS] {
	return TFPlannedState[CTFS]{}
}

func (b *BridgeV2[CPS, CTFS]) pulumiInputToTFConfig(PulumiInput[CPS]) TFConfig[CTFS] {
	return TFConfig[CTFS]{}
}

func (b *BridgeV2[CPS, CTFS]) tfRawStateFromPulumiOutput(PulumiOutput[CPS]) TFState[CTFS] {
	return TFState[CTFS]{}
}

func (b *BridgeV2[CPS, CTFS]) Check(news PulumiInput[CPS]) PulumiInput[CPS] {
	return PulumiInput[CPS]{}
}

func (b *BridgeV2[CPS, CTFS]) CreatePreview(inputs PulumiInput[CPS]) PulumiPreviewOutput[CPS] {
	tfConfig := b.pulumiInputToTFConfig(inputs)
	emptyState := TFState[CTFS]{}
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, emptyState)
	return b.tfPlannedStateToPulumiPreviewOutput(tfPlannedState)
}

func (b *BridgeV2[CPS, CTFS]) Create(inputs PulumiInput[CPS]) PulumiOutput[CPS] {
	tfConfig := b.pulumiInputToTFConfig(inputs)
	emptyState := TFState[CTFS]{}
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, emptyState)
	tfState := b.TFProvider.ApplyResourceChange(tfConfig, emptyState, tfPlannedState)
	return b.tfStateToPulumiOutput(tfState)
}

func (b *BridgeV2[CPS, CTFS]) UpdatePreview(inputs PulumiInput[CPS], state PulumiOutput[CPS]) PulumiPreviewOutput[CPS] {
	tfConfig := b.pulumiInputToTFConfig(inputs)
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfState)
	return b.tfPlannedStateToPulumiPreviewOutput(tfPlannedState)
}

func (b *BridgeV2[CPS, CTFS]) Update(inputs PulumiInput[CPS], state PulumiOutput[CPS]) PulumiOutput[CPS] {
	tfConfig := b.pulumiInputToTFConfig(inputs)
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfState)
	tfState = b.TFProvider.ApplyResourceChange(tfConfig, tfState, tfPlannedState)
	return b.tfStateToPulumiOutput(tfState)
}

func (b *BridgeV2[CPS, CTFS]) DeletePreview(state PulumiOutput[CPS]) PulumiPreviewOutput[CPS] {
	tfConfig := TFConfig[CTFS]{}
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfState)
	return b.tfPlannedStateToPulumiPreviewOutput(tfPlannedState)
}

func (b *BridgeV2[CPS, CTFS]) Delete(state PulumiOutput[CPS]) PulumiOutput[CPS] {
	tfConfig := TFConfig[CTFS]{}
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfPlannedState := b.TFProvider.PlanResourceChange(tfConfig, tfState)
	tfState = b.TFProvider.ApplyResourceChange(tfConfig, tfState, tfPlannedState)
	return b.tfStateToPulumiOutput(tfState)
}

func (b *BridgeV2[CPS, CTFS]) Diff(oldState PulumiOutput[CPS], plannedState PulumiPreviewOutput[CPS]) (PulumiDiff[CPS], PulumiDetailedDiffV2[CPS]) {
	tfState := b.tfRawStateFromPulumiOutput(oldState)
	tfPlannedState := b.pulumiPreviewOutputToTFPlannedState(plannedState)
	tfDiff := b.TFProvider.Diff(tfState, tfPlannedState)

	translateTFDiffToPulumiDiff := func(TFDiff[CTFS]) PulumiDiff[CPS] {
		return PulumiDiff[CPS]{}
	}

	producePulumiDetailedDiff := func(
		oldState PulumiOutput[CPS],
		plannedState PulumiPreviewOutput[CPS],
	) PulumiDetailedDiffV2[CPS] {
		// The values here are in the same plane - much easier to produce a diff.
		return PulumiDetailedDiffV2[CPS]{
			OldState:     oldState,
			plannedState: plannedState,
		}
	}

	pulumiDiff := translateTFDiffToPulumiDiff(tfDiff)
	pulumiDetailedDiff := producePulumiDetailedDiff(oldState, plannedState)

	return pulumiDiff, pulumiDetailedDiff
}

// extractInputsFromOutputs is not precise and has some assumptions and approximations
// Especially when there are no past inputs
func (b *BridgeV2[CPS, CTFS]) extractInputsFromOutputsNoInputs(
	outputs PulumiOutput[CPS],
) PulumiInput[CPS] {
	// guess
	return PulumiInput[CPS]{}
}

func (b *BridgeV2[CPS, CTFS]) ReadForImport(id string) (PulumiOutput[CPS], PulumiInput[CPS]) {
	tfState := b.TFProvider.Importer(id)
	state := b.tfStateToPulumiOutput(tfState)
	inputs := b.extractInputsFromOutputsNoInputs(state)
	return state, inputs
}

func (b *BridgeV2[CPS, CTFS]) ReadForRefresh(state PulumiOutput[CPS]) (PulumiOutput[CPS]) {
	tfState := b.tfRawStateFromPulumiOutput(state)
	tfNewState := b.TFProvider.ReadResource(tfState)
	newState := b.tfStateToPulumiOutput(tfNewState)
	return newState
}

// The only method which needs to deal with old schema is UpgradeState.
type BridgeV2WithUpgrade[CPS PulumiSchema, PS PulumiSchema, CTFS TFSchema, PTS TFSchema] struct {
	BridgeV2[CPS, CTFS]
	TFProvider TFProviderWithUpgradeState[CTFS, PTS]
}

var _ PulumiProviderV2WithUpgrade[PulumiSchema, PulumiSchema] = &BridgeV2WithUpgrade[PulumiSchema, PulumiSchema, TFSchema, TFSchema]{}

func (b *BridgeV2WithUpgrade[CPS, PS, CTFS, PTS]) UpgradeState(state PulumiOutput[PS]) PulumiOutput[CPS] {
	return PulumiOutput[CPS]{}
}
