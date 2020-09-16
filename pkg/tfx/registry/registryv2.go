package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	svchost "github.com/hashicorp/terraform-svchost"
)

type V2Client struct {
	name    string
	baseURL *url.URL
}

func NewV2Client(hostname string) (*V2Client, error) {
	name, err := svchost.ForComparison(hostname)
	if err != nil {
		return nil, fmt.Errorf("%v is not a valid hostname: %w", hostname, err)
	}

	url, err := url.Parse("https://" + name.String())
	if err != nil {
		return nil, err
	}

	return &V2Client{
		name:    name.ForDisplay(),
		baseURL: url,
	}, nil
}

func DefaultV2Client() (*V2Client, error) {
	return NewV2Client(DefaultName)
}

func (c *V2Client) Name() string {
	return c.name
}

type jsonUnmarshaler struct {
	dest interface{}
}

func (j jsonUnmarshaler) UnmarshalJSON(bytes []byte) error {
	return json.Unmarshal(bytes, j.dest)
}

func unmarshaler(v interface{}) json.Unmarshaler {
	return jsonUnmarshaler{v}
}

type jsonApiRepsonse struct {
	Data     json.Unmarshaler `json:"data"`
	Included json.Unmarshaler `json:"included"`
}

type jsonApiLink struct {
	Href string
	Meta map[string]interface{}
}

func (j *jsonApiLink) UnmarshalJSON(bytes []byte) error {
	var link interface{}
	if err := json.Unmarshal(bytes, &link); err != nil {
		return err
	}
	if link, ok := link.(string); ok {
		j.Href = link
		return nil
	}

	var linkObject struct {
		Href string                 `json:"href"`
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.Unmarshal(bytes, &linkObject); err != nil {
		return err
	}
	j.Href, j.Meta = linkObject.Href, linkObject.Meta
	return nil
}

type jsonApiObject struct {
	Type  string                  `json:"type"`
	ID    string                  `json:"id"`
	Links map[string]*jsonApiLink `json:"links"`
}

func (o jsonApiObject) SelfLink() (string, bool) {
	l, ok := o.Links["self"]
	if !ok {
		return "", false
	}
	return l.Href, true
}

type providerVersionObject struct {
	jsonApiObject

	Attributes ProviderVersion `json:"attributes"`
}

type ProviderVersion struct {
	Link    string `json:"-"`
	Version string `json:"version"`
}

type documentationObject struct {
	jsonApiObject

	Attributes Documentation `json:"attributes"`
}

type Documentation struct {
	Category    string `json:"category"`
	Content     string `json:"content"`
	Path        string `json:"path"`
	Slug        string `json:"slug"`
	Subcategory string `json:"subcategory"`
	Title       string `json:"title"`
	Truncated   bool   `json:"truncated"`
}

func (c *V2Client) getProviderLink(namespace, name string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%v/v2/providers?filter[namespace]=%v&filter[name]=%v", c.baseURL, namespace, name))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request failed: %v", resp.Status)
	}

	var response struct {
		Data []jsonApiObject `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	providers := response.Data
	switch len(providers) {
	case 0:
		return "", nil
	case 1:
		// OK
	default:
		return "", fmt.Errorf("more than one provider exists at %v/%v", namespace, name)
	}

	self, ok := providers[0].SelfLink()
	if !ok {
		return "", fmt.Errorf("malformed response: no self link")
	}
	return self, nil
}

func (c *V2Client) ListVersions(namespace, name string) ([]ProviderVersion, error) {
	provider, err := c.getProviderLink(namespace, name)
	if err != nil || provider == "" {
		return nil, err
	}

	resp, err := http.Get(fmt.Sprintf("%v/%v?include=provider-versions", c.baseURL, provider))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %v", resp.Status)
	}

	var response struct {
		Included []providerVersionObject
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	versions := make([]ProviderVersion, len(response.Included))
	for i, obj := range response.Included {
		self, ok := obj.SelfLink()
		if !ok {
			return nil, fmt.Errorf("malformed response: no self link for version %v", obj.Attributes.Version)
		}

		versions[i] = obj.Attributes
		versions[i].Link = self
	}

	return versions, nil
}

func (c *V2Client) GetProviderVersion(namespace, name, version string) (*ProviderVersion, error) {
	versions, err := c.ListVersions(namespace, name)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Version == version {
			return &v, nil
		}
	}
	return nil, nil
}

func (c *V2Client) GetProviderDocs(namespace, name, version string) ([]Documentation, error) {
	providerVersion, err := c.GetProviderVersion(namespace, name, version)
	if err != nil || providerVersion == nil {
		return nil, err
	}

	return c.GetProviderVersionDocs(*providerVersion)
}

func (c *V2Client) GetProviderVersionDocs(version ProviderVersion) ([]Documentation, error) {
	resp, err := http.Get(fmt.Sprintf("%v/%v?include=provider-docs", c.baseURL, version.Link))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %v", resp.Status)
	}

	var response struct {
		Included []documentationObject
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	docs := make([]Documentation, len(response.Included))
	for i, doc := range response.Included {
		docs[i] = doc.Attributes
	}
	return docs, nil
}
