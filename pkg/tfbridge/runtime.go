package tfbridge

// Holds runtime flags
type RuntimeInfo struct {
	IsTfgen bool
}

var runtime = RuntimeInfo{
	IsTfgen: false,
}

func ReadRuntimeInfo() RuntimeInfo {
	return runtime
}

func SetRuntimeIsTfgen() {
	runtime.IsTfgen = true
}
