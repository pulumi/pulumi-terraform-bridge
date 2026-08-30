package protocolspec

type PulumiDetailedDiffV2[CS PulumiSchema] struct {
	OldState     PulumiOutput[CS]
	plannedState PulumiPreviewOutput[CS]
}

// Note that the only provider method that needs to deal with old schema is UpgradeState.
// Note also that old inputs are not needed for any method.
type PulumiProviderV2[CS PulumiSchema] interface {
	// Check shouldn't apply defaults, so old Inputs are not needed. Checked input then becomes the same as input but validated.
	Check(news PulumiInput[CS]) PulumiInput[CS]

	CreatePreview(inputs PulumiInput[CS]) PulumiPreviewOutput[CS]
	// Should we go further here and avoid replanning? What is needed to achieve this?
	Create(inputs PulumiInput[CS]) PulumiOutput[CS]

	// Old inputs are not needed for Update and are currently unused.
	UpdatePreview(inputs PulumiInput[CS], upgradedState PulumiOutput[CS]) PulumiPreviewOutput[CS]
	// Avoid replanning?
	Update(inputs PulumiInput[CS], upgradedState PulumiOutput[CS]) PulumiOutput[CS]

	DeletePreview(upgradedState PulumiOutput[CS]) PulumiPreviewOutput[CS]
	// Avoid replanning?
	Delete(upgradedState PulumiOutput[CS]) PulumiOutput[CS]

	Diff(oldUpgradedState PulumiOutput[CS], plannedState PulumiPreviewOutput[CS]) (PulumiDiff[CS], PulumiDetailedDiffV2[CS])

	ReadForImport(id string) (PulumiOutput[CS], PulumiInput[CS])
	ReadForRefresh(state PulumiOutput[CS]) (PulumiOutput[CS])
}

type PulumiProviderV2WithUpgrade[CS PulumiSchema, PS PulumiSchema] interface {
	PulumiProviderV2[CS]
	UpgradeState(state PulumiOutput[PS]) PulumiOutput[CS]
}

func displayPreviewV2[CS PulumiSchema](
	proposedState PulumiPreviewOutput[CS], upgradedOldState PulumiOutput[CS], dd PulumiDetailedDiffV2[CS],
) {
}
