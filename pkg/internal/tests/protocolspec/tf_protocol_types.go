package protocolspec

type TFSchema interface{}

// TFConfig is the user provided config
type TFConfig[S TFSchema] struct{}

// TFState is the current state of the resource
type TFState[S TFSchema] struct{}

// TFPlannedState is the planned state of the resource. This applies defaults and can contain unknowns.
type TFPlannedState[S TFSchema] struct{}

// TFDiff contains the diff/ no_diff decision as well as the replacement decision
type TFDiff[S TFSchema] struct{}

// CS stands for Current Schema
type TFProvider[CS TFSchema] interface {
	// PlanResourceChange also takes a proposed state but that is just a function of the config and the state
	PlanResourceChange(config TFConfig[CS], state TFState[CS]) TFPlannedState[CS]

	ApplyResourceChange(config TFConfig[CS], state TFState[CS], planned TFPlannedState[CS]) TFState[CS]

	Diff(state TFState[CS], planned TFPlannedState[CS]) TFDiff[CS]

	Importer(id string) TFState[CS]

	ReadResource(TFState[CS]) TFState[CS]
}

// CS stands for Current Schema
// PS stands for Past Schema
type TFProviderWithUpgradeState[CS TFSchema, PS TFSchema] interface {
	TFProvider[CS]
	UpgradeState(state TFState[PS]) TFState[CS]
	GetCurrentProvider() TFProviderWithUpgradeState[CS, CS]
}
