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

var runtime = initRuntimeInfo()

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

func readRuntimeInfo() runtimeInfo {
	return runtime
}

func getRuntimeStage() runtimeStage {
	return runtime.stage
}

func setRuntimeStage(s runtimeStage) {
	runtime.stage = s
}
