package tfbridge

import (
	"runtime/debug"
	"strings"
)

// Used internally for the code to distinguish if it is running at build-time or at runtime.
type runtimeStage int

const (
	unknownStage runtimeStage = iota
	buildingProviderStage
	runningProviderStage
)

var currentRuntimeStage = guessRuntimeStage()

func guessRuntimeStage() runtimeStage {
	buildInfo, _ := debug.ReadBuildInfo()
	stage := unknownStage
	if buildInfo != nil {
		if strings.Contains(buildInfo.Path, "pulumi-tfgen") {
			stage = buildingProviderStage
		} else if strings.Contains(buildInfo.Path, "pulumi-resource") {
			stage = runningProviderStage
		}
	}
	return stage
}
