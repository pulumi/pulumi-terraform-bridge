package protocolspec

type PulumiSchema interface{}

type PulumiInput[S PulumiSchema] struct{}

type PulumiCheckedInput[S PulumiSchema] struct{}

type PulumiOutput[S PulumiSchema] struct{}

type PulumiPreviewOutput[S PulumiSchema] struct{}

type PulumiDiff[S PulumiSchema] struct {
	HasChanges bool
}

type PulumiDetailedDiff[CS PulumiSchema, PS PulumiSchema] struct {
	oldState PulumiOutput[PS]
	// Note that the PulumiDetailedDiff is expressed in terms of old state and new checked inputs.
	// This creates a lot of complexity in the provider.
	newCheckedInputs PulumiCheckedInput[CS]
}

// CS stands for Current Schema
// PS stands for Previous Schema
type PulumiProvider[CS PulumiSchema, PS PulumiSchema] interface {
	Check(news PulumiInput[CS], olds PulumiCheckedInput[PS]) PulumiCheckedInput[CS]

	CreatePreview(checkedInputs PulumiCheckedInput[CS]) PulumiPreviewOutput[CS]
	Create(checkedInputs PulumiCheckedInput[CS]) PulumiOutput[CS]

	UpdatePreview(
		checkedInputs PulumiCheckedInput[CS],
		state PulumiOutput[PS],
		oldInputs PulumiCheckedInput[PS],
	) PulumiPreviewOutput[CS]
	Update(
		checkedInputs PulumiCheckedInput[CS],
		state PulumiOutput[PS],
		oldInputs PulumiCheckedInput[PS],
	) PulumiOutput[CS]

	DeletePreview(state PulumiOutput[PS])
	Delete(state PulumiOutput[PS])

	Diff(
		oldState PulumiOutput[PS],
		oldInputs PulumiCheckedInput[PS],
		newInputs PulumiCheckedInput[CS],
	) (PulumiDiff[CS], PulumiDetailedDiff[CS, PS])

	ReadForImport(id string) (PulumiOutput[CS], PulumiInput[CS])

	ReadForRefresh(inputs PulumiCheckedInput[CS], state PulumiOutput[CS]) (PulumiInput[CS], PulumiOutput[CS])

	// TODO Read for .get
}

func displayPreview[CS PulumiSchema, PS PulumiSchema](
	inputs PulumiCheckedInput[CS], oldState PulumiOutput[PS], dd PulumiDetailedDiff[CS, PS],
) {
}
