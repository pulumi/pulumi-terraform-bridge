package util

import (
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type AliasingResourceMap interface {
	shim.ResourceMap
	RangeAliases(each func(key, value string) bool)
}

type aliasingMap struct {
	inner   shim.ResourceMap
	aliases map[string]string
}

// A resource map that only supports modification by aliasing (Set is a panic)
// and that can export all the aliases applied
func NewAliasingResourceMap(inner shim.ResourceMap) AliasingResourceMap {
	return &aliasingMap{inner: inner, aliases: make(map[string]string)}
}

func (a *aliasingMap) Get(key string) shim.Resource {
	r, _ := a.GetOk(key)
	return r
}

func (a *aliasingMap) GetOk(key string) (shim.Resource, bool) {
	if otherKey, ok := a.aliases[key]; ok {
		key = otherKey
	}
	return a.inner.GetOk(key)
}

func (a *aliasingMap) Range(each func(key string, value shim.Resource) bool) {
	a.inner.Range(func(key string, value shim.Resource) bool {
		if _, ok := a.aliases[key]; !ok {
			return each(key, value)
		}
		return true
	})
	a.RangeAliases(func(alias, target string) bool {
		return each(alias, a.inner.Get(target))
	})
}

func (a *aliasingMap) Len() int {
	n := a.inner.Len()
	a.RangeAliases(func(k, v string) bool {
		if _, ok := a.inner.GetOk(k); !ok {
			n = n + 1
		}
		return true
	})
	return n
}

func (a *aliasingMap) Set(key string, value shim.Resource) {
	panic("AliasingResourceMap does not allow Set")
}

func (a *aliasingMap) RangeAliases(each func(alias, target string) bool) {
	for alias, target := range a.aliases {
		if !each(alias, target) {
			return
		}
	}
}

func (a *aliasingMap) AddAlias(alias, target string) {
	a.aliases[alias] = target
}
