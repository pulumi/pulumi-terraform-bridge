package providercache

import (
	"github.com/opentofu/opentofu/internal/getproviders"
	"github.com/opentofu/opentofu/internal/providercache"
)

type Dir = providercache.Dir

type Installer = providercache.Installer

func NewInstaller(targetDir *Dir, source getproviders.Source) *Installer {
	return providercache.NewInstaller(targetDir, source)
}

func NewDir(path string) *Dir { return providercache.NewDir(path) }
