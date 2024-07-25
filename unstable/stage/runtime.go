package stage

import (
	"runtime/debug"
	"strings"
)

// IsTfgen returns true if the provider thinks it is running as part of `make tfgen`.
//
// The correctness of calling code should not depend on this result.
//
// This is an unstable API and should not be used outside of this repo.
func IsTfgen() bool { return currentRuntimeStage == buildingProviderStage }

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
