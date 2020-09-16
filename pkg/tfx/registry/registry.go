package registry

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	svchost "github.com/hashicorp/terraform-svchost"
	"github.com/hashicorp/terraform-svchost/disco"
)

const DefaultName = "registry.terraform.io"

type Client struct {
	name    string
	baseURL *url.URL
}

func NewClient(hostname string) (*Client, error) {
	w := log.Writer()
	log.SetOutput(ioutil.Discard)
	defer func() {
		log.SetOutput(w)
	}()

	name, err := svchost.ForComparison(hostname)
	if err != nil {
		return nil, fmt.Errorf("%v is not a valid hostname: %w", hostname, err)
	}

	host, err := disco.New().Discover(name)
	if err != nil {
		return nil, fmt.Errorf("service discovery failed: %w", err)
	}

	url, err := host.ServiceURL("providers.v1")
	if err != nil {
		return nil, fmt.Errorf("%v does not support the provider registry protocol", name.ForDisplay())
	}

	return &Client{name: name.ForDisplay(), baseURL: url}, nil
}

func DefaultClient() (*Client, error) {
	return NewClient(DefaultName)
}

func (c *Client) Name() string {
	return c.name
}

func supportsProtocolVersion(vs string, major, minor int) bool {
	components := strings.SplitN(vs, ".", 2)
	if len(components) != 2 {
		return false
	}
	protocolMajor, err := strconv.ParseInt(components[0], 10, 32)
	if err != nil {
		return false
	}
	protocolMinor, err := strconv.ParseInt(components[1], 10, 32)
	if err != nil {
		return false
	}
	return int(protocolMajor) == major && int(protocolMinor) >= minor
}

type Platform struct {
	OperatingSystem string `json:"os"`
	Architecture    string `json:"arch"`
}

type VersionSummary struct {
	Version   string     `json:"version"`
	Protocols []string   `json:"protocols"`
	Platforms []Platform `json:"platforms"`
}

func (v VersionSummary) SupportsProtocolVersion(major, minor int) bool {
	for _, p := range v.Protocols {
		if supportsProtocolVersion(p, major, minor) {
			return true
		}
	}
	return false
}

func (v VersionSummary) SupportsPlatform(os, arch string) bool {
	for _, p := range v.Platforms {
		if p.OperatingSystem == os && p.Architecture == arch {
			return true
		}
	}
	return false
}

func (c *Client) ListVersions(namespace, name string) ([]VersionSummary, error) {
	resp, err := http.Get(fmt.Sprintf("%v/%s/%s/versions", c.baseURL, namespace, name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %v", resp.Status)
	}

	var versions struct {
		Versions []VersionSummary `json:"versions"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, err
	}
	return versions.Versions, nil
}

type GPGPublicKey struct {
	KeyID          string `json:"key_id"`
	ASCIIArmor     string `json:"ascii_armor"`
	TrustSignature string `json:"trust_signature"`
	Source         string `json:"source"`
	SourceURL      string `json:"source_url"`
}

type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys"`
}

type VersionMetadata struct {
	Protocols           []string    `json:"protocols"`
	OperatingSystem     string      `json:"os"`
	Architecture        string      `json:"arch"`
	Filename            string      `json:"filename"`
	URL                 string      `json:"download_url"`
	SHASumsURL          string      `json:"shasums_url"`
	SHASumsSignatureURL string      `json:"shasums_signature_url"`
	SHASum              string      `json:"shasum"`
	SigningKeys         SigningKeys `json:"signing_keys"`
}

func (v VersionMetadata) SupportsProtocolVersion(major, minor int) bool {
	for _, p := range v.Protocols {
		if supportsProtocolVersion(p, major, minor) {
			return true
		}
	}
	return false
}

func (c *Client) GetVersion(namespace, name, version, os, arch string) (VersionMetadata, error) {
	resp, err := http.Get(fmt.Sprintf("%v/%s/%s/%s/download/%s/%s", c.baseURL, namespace, name, version, os, arch))
	if err != nil {
		return VersionMetadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return VersionMetadata{}, fmt.Errorf("request failed: %v", resp.Status)
	}

	var versionMeta VersionMetadata
	if err = json.NewDecoder(resp.Body).Decode(&versionMeta); err != nil {
		return VersionMetadata{}, err
	}
	return versionMeta, nil
}
