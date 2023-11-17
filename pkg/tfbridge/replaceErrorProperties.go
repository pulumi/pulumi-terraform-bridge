package tfbridge

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

func replaceSubstring(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

func ReplaceSubstrings(s string, replacements map[string]string) string {
	for k, v := range replacements {
		s = replaceSubstring(s, k, v)
	}
	return s
}

func getConfigReplacements(info ProviderInfo) (map[string]string, error) {
	var renames map[string]interface{}
	if r, found, err := metadata.Get[map[string]interface{}](info.MetadataInfo.Data, "renames"); err != nil {
		return nil, errors.Wrap(err, "getting renames failed")
	} else if found {
		renames = r
	} else {
		return nil, errors.Wrap(fmt.Errorf("missing pre-computed muxer renames"), "")
	}

	renamedConfigProperties := renames["renamedConfigProperties"]
	var configPropertyRenames map[string]string = make(map[string]string)
	for k, v := range renamedConfigProperties.(map[string]interface{}) {
		// The metadata has the renames from pulumi -> tf, so we need to invert it.
		configPropertyRenames[fmt.Sprint(v)] = info.Name + ":" + k
	}
	return configPropertyRenames, nil
}

func ReplaceErrorProperties(err error, info ProviderInfo) error {
	replacements, replacementsErr := getConfigReplacements(info)
	if replacementsErr != nil {
		return multierror.Append(err, replacementsErr)
	}
	return errors.New(ReplaceSubstrings(err.Error(), replacements))
}
