package plugins

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/mholt/archiver/v3"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/registry"
)

const DefaultNamespace = "hashicorp"

type PluginMeta struct {
	RegistryName   string
	Namespace      string
	Name           string
	Version        *semver.Version
	ExecutablePath string
}

func (p PluginMeta) String() string {
	return p.Reference()
}

func (p PluginMeta) Reference() string {
	registryName, namespace, name := url.PathEscape(p.RegistryName), url.PathEscape(p.Namespace), url.PathEscape(p.Name)

	var components []string
	switch {
	case p.RegistryName != registry.DefaultName:
		components = []string{registryName, namespace, name}
	case p.Namespace != DefaultNamespace:
		components = []string{namespace, name}
	default:
		components = []string{name}
	}
	ref := strings.Join(components, "/")

	if p.Version != nil {
		ref += "@" + p.Version.String()
	}

	return ref
}

func ParsePluginReference(ref string) (*PluginMeta, error) {
	var meta PluginMeta

	atIndex := strings.IndexRune(ref, '@')
	if atIndex != -1 {
		versionString, err := url.PathUnescape(ref[atIndex+1:])
		if err != nil {
			return nil, fmt.Errorf("failed to unescape version: %w", err)
		}
		v, err := semver.ParseTolerant(versionString)
		if err != nil {
			return nil, fmt.Errorf("invalid version: %w", err)
		}
		meta.Version, ref = &v, ref[:atIndex]
	}

	components := strings.Split(ref, "/")
	switch len(components) {
	case 1:
		meta.RegistryName = registry.DefaultName
		meta.Namespace = DefaultNamespace
		meta.Name = components[0]
	case 2:
		meta.RegistryName = registry.DefaultName
		meta.Namespace = components[0]
		meta.Name = components[1]
	case 3:
		meta.RegistryName = components[0]
		meta.Namespace = components[1]
		meta.Name = components[2]
	default:
		return nil, fmt.Errorf("plugin references must be of the form '[[<registry>/]<namespace>/]<name>[@<version>]'")
	}

	var err error
	meta.RegistryName, err = url.PathUnescape(meta.RegistryName)
	if err != nil {
		return nil, fmt.Errorf("failed to unescape registry name: %w", err)
	}
	meta.Namespace, err = url.PathUnescape(meta.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to unescape namespace: %w", err)
	}
	meta.Name, err = url.PathUnescape(meta.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to unescape name: %w", err)
	}

	return &meta, nil
}

type ProgressFunc func(closer io.ReadCloser, size int64, message string) io.ReadCloser

type Cache struct {
	rootPath string
}

func DefaultCache() (*Cache, error) {
	cacheDir, err := workspace.GetPulumiPath("tfx-plugins")
	if err != nil {
		return nil, err
	}
	return &Cache{rootPath: cacheDir}, nil
}

func (c *Cache) findPluginExecutable(versionPath string) (string, bool) {
	entries, err := ioutil.ReadDir(versionPath)
	if err != nil {
		return "", false
	}

	executable := ""
	for _, info := range entries {
		if info.Mode().Perm()&0111 != 0 {
			if executable != "" {
				return "", false
			}
			executable = info.Name()
		}
	}
	return executable, true
}

func (c *Cache) pluginDir(r *registry.Client, namespace, name string) string {
	return filepath.Join(c.rootPath, r.Name(), namespace, name)
}

func (c *Cache) versionDir(r *registry.Client, namespace, name, version string) string {
	return filepath.Join(c.pluginDir(r, namespace, name), version)
}

func (c *Cache) findInstalledPlugin(r *registry.Client, namespace, name string, inRange semver.Range) (*PluginMeta, error) {
	pluginDir := c.pluginDir(r, namespace, name)

	entries, err := ioutil.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var best *semver.Version
	var bestExecutablePath string
	for _, info := range entries {
		if !info.IsDir() {
			continue
		}

		sv, err := semver.ParseTolerant(info.Name())
		if err != nil || !inRange(sv) {
			continue
		}

		if best == nil || sv.GT(*best) {
			versionDir := filepath.Join(pluginDir, info.Name())
			if executable, ok := c.findPluginExecutable(versionDir); ok {
				best, bestExecutablePath = &sv, filepath.Join(versionDir, executable)
			}
		}
	}
	if best == nil {
		return nil, nil
	}
	return &PluginMeta{
		RegistryName:   r.Name(),
		Namespace:      namespace,
		Name:           name,
		Version:        best,
		ExecutablePath: bestExecutablePath,
	}, nil
}

func (c *Cache) getPluginVersionFromRegistry(r *registry.Client, namespace, name string,
	inRange semver.Range) (semver.Version, *registry.VersionMetadata, error) {

	versions, err := r.ListVersions(namespace, name)
	if err != nil {
		return semver.Version{}, nil, err
	}

	var best *semver.Version
	var version string
	for _, v := range versions {
		sv, err := semver.ParseTolerant(v.Version)
		if err != nil || !inRange(sv) {
			continue
		}
		if best == nil || sv.GT(*best) {
			best, version = &sv, v.Version
		}
	}
	if best == nil {
		return semver.Version{}, nil, fmt.Errorf("no matching version found in registry")
	}

	versionMeta, err := r.GetVersion(namespace, name, version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return semver.Version{}, nil, err
	}
	return *best, &versionMeta, nil
}

func (c *Cache) installPlugin(r *registry.Client, namespace, name string, inRange semver.Range, progress ProgressFunc) (*PluginMeta, error) {
	version, versionMeta, err := c.getPluginVersionFromRegistry(r, namespace, name, inRange)
	if err != nil {
		return nil, err
	}
	if !versionMeta.SupportsProtocolVersion(5, 0) {
		return nil, fmt.Errorf("%v does not support protocol version 5.0", &PluginMeta{
			RegistryName: r.Name(),
			Namespace:    namespace,
			Name:         name,
			Version:      &version,
		})
	}

	tempDir, err := ioutil.TempDir("", "tfx-"+name)
	if err != nil {
		return nil, err
	}
	removeDir := true
	defer func() {
		if removeDir {
			err := os.RemoveAll(tempDir)
			contract.IgnoreError(err)
		}
	}()

	zipFilePath := filepath.Join(tempDir, "provider.zip")
	extractedPath := filepath.Join(tempDir, "provider")
	downloadFile := func() error {
		zipFile, err := os.Create(zipFilePath)
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(zipFile)

		resp, err := http.Get(versionMeta.URL)
		if err != nil {
			return err
		}
		body := resp.Body
		if progress != nil {
			body = progress(body, resp.ContentLength, fmt.Sprintf("Downloading %v", &PluginMeta{
				RegistryName: r.Name(),
				Namespace:    namespace,
				Name:         name,
				Version:      &version,
			}))
		}
		defer contract.IgnoreClose(body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("request failed: %v", resp.Status)
		}

		_, err = io.Copy(zipFile, body)
		return err
	}
	if err = downloadFile(); err != nil {
		return nil, err
	}
	if err := archiver.NewZip().Unarchive(zipFilePath, extractedPath); err != nil {
		return nil, err
	}
	executable, ok := c.findPluginExecutable(extractedPath)
	if !ok {
		return nil, fmt.Errorf("provider %v is missing a plugin executable", name)
	}

	if err = os.MkdirAll(c.pluginDir(r, namespace, name), 0700); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("failed to create plugin directory: %v", err)
	}

	versionDir := c.versionDir(r, namespace, name, version.String())
	if err = os.Rename(extractedPath, versionDir); err != nil {
		// Assume that another installer beat us to the rename.
		exec, ok := c.findPluginExecutable(versionDir)
		if !ok {
			return nil, fmt.Errorf("version directory %v is missing a plugin executable", versionDir)
		}
		executable = exec
	} else {
		removeDir = false
	}

	return &PluginMeta{
		RegistryName:   r.Name(),
		Namespace:      namespace,
		Name:           name,
		Version:        &version,
		ExecutablePath: filepath.Join(versionDir, executable),
	}, nil
}

func (c *Cache) GetPlugin(r *registry.Client, namespace, name string, inRange semver.Range) (*PluginMeta, error) {
	return c.findInstalledPlugin(r, namespace, name, inRange)
}

func (c *Cache) EnsurePlugin(r *registry.Client, namespace, name string, inRange semver.Range, progress ProgressFunc) (*PluginMeta, error) {
	meta, err := c.findInstalledPlugin(r, namespace, name, inRange)
	if err != nil || meta != nil {
		return meta, err
	}
	return c.installPlugin(r, namespace, name, inRange, progress)
}

func rangeDir(path string, each func(info os.FileInfo) error) error {
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	for _, info := range entries {
		if err = each(info); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) ListAllPlugins() ([]PluginMeta, error) {
	var plugins []PluginMeta
	if err := rangeDir(c.rootPath, func(registryInfo os.FileInfo) error {
		registryName := registryInfo.Name()
		registryPath := filepath.Join(c.rootPath, registryName)
		return rangeDir(registryPath, func(namespaceInfo os.FileInfo) error {
			namespace := namespaceInfo.Name()
			namespacePath := filepath.Join(registryPath, namespace)
			return rangeDir(namespacePath, func(nameInfo os.FileInfo) error {
				name := nameInfo.Name()
				namePath := filepath.Join(namespacePath, name)
				return rangeDir(namePath, func(versionInfo os.FileInfo) error {
					version, err := semver.ParseTolerant(versionInfo.Name())
					if err != nil {
						// Ignore versions that fail to parse.
						return nil
					}

					versionPath := filepath.Join(namePath, versionInfo.Name())
					executable, ok := c.findPluginExecutable(versionPath)
					if !ok {
						// Ignore missing executables.
						return nil
					}

					plugins = append(plugins, PluginMeta{
						RegistryName:   registryName,
						Namespace:      namespace,
						Name:           name,
						Version:        &version,
						ExecutablePath: filepath.Join(versionPath, executable),
					})
					return nil
				})
			})
		})
	}); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return plugins, nil
}
