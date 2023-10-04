package tfbridge

import (
	"runtime/debug"
	"strings"
)

type RuntimeStage int

const (
	UnknownStage RuntimeStage = iota
	TfgenStage
	ResourceStage
)

// Holds runtime flags
type RuntimeInfo struct {
	Stage RuntimeStage
}

var runtime = initRuntimeInfo()

func initRuntimeInfo() RuntimeInfo {
	buildInfo, _ := debug.ReadBuildInfo()
	stage := UnknownStage
	if buildInfo != nil {
		if strings.Contains(buildInfo.Path, "pulumi-tfgen") {
			stage = TfgenStage
		} else if strings.Contains(buildInfo.Path, "pulumi-resource") {
			stage = ResourceStage
		}
	}
	return RuntimeInfo{
		Stage: stage,
	}
}

func ReadRuntimeInfo() RuntimeInfo {
	return runtime
}

func GetRuntimeStage() RuntimeStage {
	return runtime.Stage
}

func SetRuntimeStage(s RuntimeStage) {
	runtime.Stage = s
}
