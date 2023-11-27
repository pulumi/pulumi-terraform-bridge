package tfbridge

import (
	"runtime/debug"
	"strings"
)

type runtimeStage int

const (
	unknownStage runtimeStage = iota
	tfgenStage
	resourceStage
)

// Holds runtime flags
type runtimeInfo struct {
	stage runtimeStage
}

var theRuntimeInfo = initRuntimeInfo()

func initRuntimeInfo() runtimeInfo {
	buildInfo, _ := debug.ReadBuildInfo()
	stage := unknownStage
	if buildInfo != nil {
		if strings.Contains(buildInfo.Path, "pulumi-tfgen") {
			stage = tfgenStage
		} else if strings.Contains(buildInfo.Path, "pulumi-resource") {
			stage = resourceStage
		}
	}
	return runtimeInfo{
		stage: stage,
	}
}

func isTfgen() bool {
	return theRuntimeInfo.stage != resourceStage
}
