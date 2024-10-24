package attribute_validator

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// urlWithSchemeAttributeValidator checks that a types.String attribute
// is indeed a URL and its scheme is one of the given `acceptableSchemes`.
//
// Instances should be created via UrlWithScheme function.
type urlWithSchemeAttributeValidator struct {
	acceptableSchemes []string
}

// UrlWithScheme is a helper to instantiate a urlWithSchemeAttributeValidator.
func UrlWithScheme(acceptableSchemes ...string) validator.String {
	return &urlWithSchemeAttributeValidator{acceptableSchemes}
}

var _ validator.String = (*urlWithSchemeAttributeValidator)(nil)

func (av *urlWithSchemeAttributeValidator) Description(ctx context.Context) string {
	return av.MarkdownDescription(ctx)
}

func (av *urlWithSchemeAttributeValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Ensures that the attribute is a URL and its scheme is one of: %q", av.acceptableSchemes)
}

func (av *urlWithSchemeAttributeValidator) ValidateString(ctx context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	tflog.Debug(ctx, "Validating attribute value is a URL with acceptable scheme", map[string]interface{}{
		"attribute":         req.Path.String(),
		"acceptableSchemes": strings.Join(av.acceptableSchemes, ","),
	})

	u, err := url.Parse(req.ConfigValue.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid URL",
			fmt.Sprintf("Parsing URL %q failed: %v", req.ConfigValue.ValueString(), err),
		)
		return
	}

	if u.Host == "" {
		res.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid URL",
			fmt.Sprintf("URL %q contains no host", u.String()),
		)
		return
	}

	for _, s := range av.acceptableSchemes {
		if u.Scheme == s {
			return
		}
	}

	res.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid URL scheme",
		fmt.Sprintf("URL %q expected to use scheme from %q, got: %q", u.String(), av.acceptableSchemes, u.Scheme),
	)
}
