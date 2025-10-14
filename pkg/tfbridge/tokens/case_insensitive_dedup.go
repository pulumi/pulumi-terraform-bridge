package tokens

import (
	"fmt"
	"strings"

	ptokens "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// WithCaseInsensitiveDedup wraps an existing strategy and rewrites automatically generated
// tokens whose names collide when compared case-insensitively. The first occurrence of a
// token is preserved; subsequent collisions receive a deterministic V suffix. The
// provided finalize function must be the same one used by the underlying strategy (e.g.
// tokens.MakeStandard). When provider metadata is supplied, previously emitted tokens are
// replayed from the auto-alias history so that the same resource keeps the unsuffixed name
// across multiple tfgen runs—a necessity for stable published providers.
func WithCaseInsensitiveDedup(base Strategy, finalize Make, metadata info.ProviderMetadata) Strategy {
	var result Strategy

	if base.Resource != nil {
		resourceDeduper := newStandardTokenDeduper(finalize, metadata, deduperKindResource)
		result.Resource = func(tfToken string, elem *info.Resource) error {
			if err := resourceDeduper.Err(); err != nil {
				return err
			}
			if elem != nil && elem.Tok == "" {
				if prior, ok := resourceDeduper.priorToken(tfToken); ok {
					elem.Tok = ptokens.Type(prior)
				}
			}
			var userOverride string
			// Remember user-specified tokens (and prior tokens) so we don't rewrite them
			if elem != nil && elem.Tok != "" {
				userOverride = string(elem.Tok)
				resourceDeduper.register(userOverride)
			}
			if err := base.Resource(tfToken, elem); err != nil {
				return err
			}
			// If after running the Strategy we still don't have a token, then
			// we are done
			if elem == nil || elem.Tok == "" {
				return nil
			}
			if userOverride != "" {
				current := string(elem.Tok)
				// None of the current built-in strategies will override a user provided token, but
				// we should still handle this in case users provide one that does
				if userOverride != current {
					resourceDeduper.unregister(userOverride)
					resourceDeduper.register(current)
				}
				return nil
			}
			tok, err := resourceDeduper.ensureUnique(string(elem.Tok))
			if err != nil {
				return fmt.Errorf("resource %q: %w", tfToken, err)
			}
			elem.Tok = ptokens.Type(tok)
			return nil
		}
	}

	if base.DataSource != nil {
		dataSourceDeduper := newStandardTokenDeduper(finalize, metadata, deduperKindDataSource)
		result.DataSource = func(tfToken string, elem *info.DataSource) error {
			if err := dataSourceDeduper.Err(); err != nil {
				return err
			}
			if elem != nil && elem.Tok == "" {
				if prior, ok := dataSourceDeduper.priorToken(tfToken); ok {
					elem.Tok = ptokens.ModuleMember(prior)
				}
			}
			var userOverride string
			if elem != nil && elem.Tok != "" {
				userOverride = string(elem.Tok)
				dataSourceDeduper.register(userOverride)
			}
			if err := base.DataSource(tfToken, elem); err != nil {
				return err
			}
			if elem == nil || elem.Tok == "" {
				return nil
			}
			if userOverride != "" {
				current := string(elem.Tok)
				if userOverride != current {
					dataSourceDeduper.unregister(userOverride)
					dataSourceDeduper.register(current)
				}
				return nil
			}
			tok, err := dataSourceDeduper.ensureUnique(string(elem.Tok))
			if err != nil {
				return fmt.Errorf("datasource %q: %w", tfToken, err)
			}
			elem.Tok = ptokens.ModuleMember(tok)
			return nil
		}
	}

	return result
}

type standardTokenDeduper struct {
	finalize           Make
	nextVariant        map[string]int
	metadata           info.ProviderMetadata
	lastPublishedToken map[string]string
	err                error
	kind               deduperKind
}

const firstVariant = 2

const aliasMetadataKey = "auto-aliasing"

type deduperKind int

const (
	deduperKindResource deduperKind = iota
	deduperKindDataSource
)

type aliasHistorySnapshot struct {
	Resources   map[string]aliasTokenSnapshot `json:"resources,omitempty"`
	DataSources map[string]aliasTokenSnapshot `json:"datasources,omitempty"`
}

type aliasTokenSnapshot struct {
	Current string `json:"current"`
}

// newStandardTokenDeduper constructs a deduper for either resources or data sources and
// immediately seeds it from persisted metadata when available.
func newStandardTokenDeduper(finalize Make, metadata info.ProviderMetadata, kind deduperKind) *standardTokenDeduper {
	d := &standardTokenDeduper{
		finalize:           finalize,
		nextVariant:        map[string]int{},
		metadata:           metadata,
		lastPublishedToken: map[string]string{},
		kind:               kind,
	}
	d.seedFromMetadata()
	return d
}

// seedFromMetadata replays the token chosen in prior runs so the same TF token keeps the
// unsuffixed name and the counter continues from the last V suffix.
func (d *standardTokenDeduper) seedFromMetadata() {
	if d.metadata == nil || d.err != nil {
		return
	}
	history, ok, err := md.Get[aliasHistorySnapshot](d.metadata, aliasMetadataKey)
	if err != nil {
		d.err = fmt.Errorf("reading %q metadata: %w", aliasMetadataKey, err)
		return
	}
	if !ok {
		return
	}

	var entries map[string]aliasTokenSnapshot
	switch d.kind {
	case deduperKindResource:
		entries = history.Resources
	case deduperKindDataSource:
		entries = history.DataSources
	default:
		return
	}
	for tfToken, snapshot := range entries {
		if snapshot.Current == "" {
			continue
		}
		// Record the last published token so that when we encounter the TF token again on
		// a future run we can reuse that identity rather than giving the unsuffixed name to
		// a newly added resource that happens to sort earlier.
		d.lastPublishedToken[tfToken] = snapshot.Current
		// Seeding the counter ensures subsequent collisions pick the next available suffix
		// instead of reusing V2 and producing churn in generated SDKs.
		d.register(snapshot.Current)
	}
}

// register notes that we have already seen a token at least once and should begin handing
// out the next variant (currently V2) on the following collision. Safe to call multiple
// times on the same token.
func (d *standardTokenDeduper) register(token string) {
	parts, ok := parseStandardToken(token)
	if !ok {
		return
	}
	key := parts.normalizedKey()
	if _, exists := d.nextVariant[key]; !exists {
		d.nextVariant[key] = firstVariant
	}
}

// unregister forgets a previously recorded token, typically because a manual override
// changed after the base strategy ran.
func (d *standardTokenDeduper) unregister(token string) {
	parts, ok := parseStandardToken(token)
	if !ok {
		return
	}
	key := parts.normalizedKey()
	delete(d.nextVariant, key)
}

// Err exposes any initialization or metadata failures so callers can stop early.
func (d *standardTokenDeduper) Err() error {
	return d.err
}

// priorToken reports the last emitted Pulumi token for the given TF token, if we seeded it
// from metadata.
func (d *standardTokenDeduper) priorToken(tfToken string) (string, bool) {
	if d.lastPublishedToken == nil {
		return "", false
	}
	tok, ok := d.lastPublishedToken[tfToken]
	return tok, ok
}

// ensureUnique hands back either the original token or a V suffix that does not
// collide case-insensitively with previously seen tokens.
func (d *standardTokenDeduper) ensureUnique(token string) (string, error) {
	if d.err != nil {
		return "", d.err
	}
	parts, ok := parseStandardToken(token)
	if !ok {
		// Token format is not recognized; leave it unchanged.
		return token, nil
	}
	key := parts.normalizedKey()
	next, exists := d.nextVariant[key]
	if !exists {
		d.register(token)
		return token, nil
	}
	variant := next
	for {
		candidateName := fmt.Sprintf("%sV%d", parts.PascalName, variant)
		newToken, err := d.finalize(parts.Module, candidateName)
		if err != nil {
			return "", err
		}
		nextParts, ok := parseStandardToken(newToken)
		if !ok {
			return "", fmt.Errorf("unexpected token format from finalize: %q", newToken)
		}
		candidateKey := nextParts.normalizedKey()
		if _, exists := d.nextVariant[candidateKey]; !exists {
			d.nextVariant[key] = variant + 1
			// Track the suffixed token as well so a future collision on the exact token
			// (case-insensitively) advances again instead of reusing the V2 name.
			d.nextVariant[candidateKey] = firstVariant
			return newToken, nil
		}
		variant++
	}
}

type tokenParts struct {
	Package    string
	Module     string
	PascalName string
}

func (p tokenParts) normalizedKey() string {
	return strings.ToLower(p.Module) + ":" + strings.ToLower(p.PascalName)
}

// parseStandardToken splits a standard Pulumi token (pkg:module/lower:Name) into its
// constituent parts; tokens outside that format are ignored by the deduper.
func parseStandardToken(token string) (tokenParts, bool) {
	if member, err := ptokens.ParseModuleMember(token); err == nil {
		qName := ptokens.IntoQName(member.Module().Name().String())
		return tokenParts{
			Package:    member.Package().String(),
			Module:     qName.Namespace().String(),
			PascalName: member.Name().String(),
		}, true
	}
	return tokenParts{}, false
}
