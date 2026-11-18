package protocolspec

func getUserInputs[S PulumiSchema]() PulumiInput[S] {
	return PulumiInput[S]{}
}

func getOldInputs[S PulumiSchema]() PulumiCheckedInput[S] {
	return PulumiCheckedInput[S]{}
}

func getOldState[S PulumiSchema]() PulumiOutput[S] {
	return PulumiOutput[S]{}
}

func CreateResource() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}
	userInputs := getUserInputs[PS]()
	oldInputs := getOldInputs[PS]()

	checkedInputs := prov.Check(userInputs, oldInputs)
	previewOutputs := prov.CreatePreview(checkedInputs)

	// previewOutputs is discarded?
	_ = previewOutputs

	// Note the engine does not display unknowns in the preview
	displayPreview(checkedInputs, PulumiOutput[PS]{}, PulumiDetailedDiff[PS, PS]{})

	prov.Create(checkedInputs)
}

func UpdateResourceNoUpgrade() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}
	userInputs := getUserInputs[PS]()
	oldState := getOldState[PS]()
	oldInputs := getOldInputs[PS]()

	checkedInputs := prov.Check(userInputs, oldInputs)

	diff, detailedDiff := prov.Diff(oldState, oldInputs, checkedInputs)
	if diff.HasChanges {
		previewOutputs := prov.UpdatePreview(checkedInputs, oldState, oldInputs)
		// previewOutputs is discarded?
		_ = previewOutputs
	}

	displayPreview(checkedInputs, oldState, detailedDiff)

	prov.Update(checkedInputs, oldState, oldInputs)
}

func UpdateResourceWithUpgrade() {
	type CPS struct{}
	type CTFS struct{}
	type PPS struct{}
	type PTFS struct{}
	prov := &Bridge[CPS, PPS, CTFS, PTFS]{}
	userInputs := getUserInputs[CPS]()
	oldState := getOldState[PPS]()
	oldInputs := getOldInputs[PPS]()

	checkedInputs := prov.Check(userInputs, oldInputs)

	diff, detailedDiff := prov.Diff(oldState, oldInputs, checkedInputs)
	if diff.HasChanges {
		previewOutputs := prov.UpdatePreview(checkedInputs, oldState, oldInputs)
		// previewOutputs is discarded?
		_ = previewOutputs
	}

	displayPreview(checkedInputs, oldState, detailedDiff)

	prov.Update(checkedInputs, oldState, oldInputs)
}

func DeleteResourceNoUpgrade() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}
	oldState := getOldState[PS]()

	prov.Delete(oldState)
}

func DeleteResourceWithUpgrade() {
	type CPS struct{}
	_ = CPS{} // unused
	type CTFS struct{}
	_ = CTFS{} // unused
	type PPS struct{}
	type PTFS struct{}

	// Note the engine currently uses the old provider for delete
	// TODO: Is this always the case?
	prov := &Bridge[PPS, PPS, PTFS, PTFS]{}
	oldState := getOldState[PPS]()

	prov.Delete(oldState)
}

func ImportResource() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}
	id := "imported-resource"

	prov.ReadForImport(id)
}

func RefreshNoUpgrade() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}

	inputs := getOldInputs[PS]()
	state := getOldState[PS]()

	readInputs, readState := prov.ReadForRefresh(inputs, state)

	readCheckedInputs := prov.Check(readInputs, inputs)

	//nolint:lll
	// parameter inversion here: https://pulumi-developer-docs.readthedocs.io/latest/developer-docs/providers/implementers-guide.html#refresh
	diff, detailedDiff := prov.Diff(readState, readCheckedInputs, inputs)
	if diff.HasChanges {
		previewOutputs := prov.UpdatePreview(readCheckedInputs, state, inputs)
		// previewOutputs is discarded?
		_ = previewOutputs
	}

	displayPreview(readCheckedInputs, state, detailedDiff)
	// state is replaced here.
}

func RefreshWithUpgrade() {
	type CPS struct{}
	_ = CPS{} // unused
	type CTFS struct{}
	_ = CTFS{} // unused
	type PPS struct{}
	type PTFS struct{}

	// Note the engine currently uses the old provider for refresh
	// TODO: Is this always the case?
	prov := &Bridge[PPS, PPS, PTFS, PTFS]{}
	state := getOldState[PPS]()
	inputs := getOldInputs[PPS]()

	readInputs, readState := prov.ReadForRefresh(inputs, state)

	readCheckedInputs := prov.Check(readInputs, inputs)

	//nolint:lll
	// parameter inversion here: https://pulumi-developer-docs.readthedocs.io/latest/developer-docs/providers/implementers-guide.html#refresh
	diff, detailedDiff := prov.Diff(readState, readCheckedInputs, inputs)
	if diff.HasChanges {
		previewOutputs := prov.UpdatePreview(readCheckedInputs, state, inputs)
		// previewOutputs is discarded?
		_ = previewOutputs
	}

	displayPreview(readCheckedInputs, state, detailedDiff)
}
