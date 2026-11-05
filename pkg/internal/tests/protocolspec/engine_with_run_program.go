package protocolspec

func RPCreateResource() {
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

func RPUpdateResourceNoUpgrade() {
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

func RPUpdateResourceWithUpgrade() {
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

func RPDeleteResourceNoUpgrade() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}
	oldState := getOldState[PS]()

	prov.Delete(oldState)
}

func RPDeleteResourceWithUpgrade() {
	type CPS struct{}
	type CTFS struct{}
	type PPS struct{}
	type PTFS struct{}

	prov := &Bridge[CPS, PPS, CTFS, PTFS]{}
	oldState := getOldState[PPS]()

	prov.Delete(oldState)
}

func RPImportResource() {
	type PS struct{}
	type TFS struct{}
	prov := &Bridge[PS, PS, TFS, TFS]{}
	id := "imported-resource"

	prov.ReadForImport(id)
}

func RPRefreshNoUpgrade() {
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

func RPRefreshWithUpgrade() {
	type CPS struct{}
	type CTFS struct{}
	type PPS struct{}
	type PTFS struct{}

	prov := &Bridge[CPS, PPS, CTFS, PTFS]{}
	state := getOldState[PPS]()
	inputs := getOldInputs[PPS]()

	readInputs, readState := prov.ReadForRefresh(inputs, state)

	readCheckedInputs := prov.Check(readInputs, inputs)

	// fudge the types here - this is fine as as a provider can upgrade from current to current.
	prov2 := prov.getCurrentBridge()

	//nolint:lll
	// parameter inversion here: https://pulumi-developer-docs.readthedocs.io/latest/developer-docs/providers/implementers-guide.html#refresh
	diff, detailedDiff := prov2.Diff(
		readState,
		readCheckedInputs,
		// We are passing old inputs to the current provider. This is very likely to be a problem.
		// https://github.com/pulumi/pulumi-terraform-bridge/issues/3027
		inputs, // cannot use inputs (variable of struct type PulumiCheckedInput[PPS]) as PulumiCheckedInput[CPS] value in argument to prov.Diff
	)
	if diff.HasChanges {
		previewOutputs := prov.UpdatePreview(readCheckedInputs, state, inputs)
		// previewOutputs is discarded?
		_ = previewOutputs
	}

	displayPreview(readCheckedInputs, state, detailedDiff)
}
