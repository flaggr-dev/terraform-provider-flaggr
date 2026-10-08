package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// redactedSecret stands in for a credential inside a metric-source or
// alert-channel config when the API token is below the admin tier (API keys,
// tokens, webhook URLs, header values). Sent back unchanged on an update, it
// keeps the stored secret; a create can't use it.
const redactedSecret = "[redacted]"

// decodeConfig parses a config document, keeping numbers exactly as written.
func decodeConfig(raw string) (interface{}, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var value interface{}
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unexpected data after the JSON value")
	}
	return value, nil
}

// mergeRedacted returns the API's config with every redacted placeholder
// replaced by the prior value at the same path (objects by key, arrays by
// index). A placeholder with no prior value behind it stays.
func mergeRedacted(api, prior interface{}) interface{} {
	switch a := api.(type) {
	case string:
		if a == redactedSecret && prior != nil {
			return prior
		}
		return a
	case map[string]interface{}:
		p, _ := prior.(map[string]interface{})
		out := make(map[string]interface{}, len(a))
		for key, value := range a {
			out[key] = mergeRedacted(value, p[key])
		}
		return out
	case []interface{}:
		p, _ := prior.([]interface{})
		out := make([]interface{}, len(a))
		for i, value := range a {
			var previous interface{}
			if i < len(p) {
				previous = p[i]
			}
			out[i] = mergeRedacted(value, previous)
		}
		return out
	default:
		return api
	}
}

// jsonEqual compares decoded JSON values; numbers compare by value.
func jsonEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for key, value := range av {
			other, found := bv[key]
			if !found || !jsonEqual(value, other) {
				return false
			}
		}
		return true
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !jsonEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case json.Number:
		bv, ok := b.(json.Number)
		if !ok {
			return false
		}
		if av == bv {
			return true
		}
		af, aerr := av.Float64()
		bf, berr := bv.Float64()
		return aerr == nil && berr == nil && af == bf
	default:
		return a == b
	}
}

// configFromAPI is the config a Read stores. Secrets the API returns as the
// redacted placeholder keep the prior state's value, so a token below the
// admin tier causes no permanent diff; when the result is the same JSON value
// as the prior state, the prior string itself is kept, so key order and
// formatting don't show as a diff either. providerKeys are top-level keys the
// provider adds to every request (the metric source "type"); they are ignored
// unless the prior config sets them. Without a usable prior state (import),
// the API's config is stored as returned.
func configFromAPI(apiConfig json.RawMessage, prior types.String, providerKeys ...string) types.String {
	raw := types.StringValue(string(apiConfig))
	if prior.IsNull() || prior.IsUnknown() {
		return raw
	}
	api, err := decodeConfig(string(apiConfig))
	if err != nil {
		return raw
	}
	previous, err := decodeConfig(prior.ValueString())
	if err != nil {
		return raw
	}
	if object, ok := api.(map[string]interface{}); ok {
		previousObject, _ := previous.(map[string]interface{})
		for _, key := range providerKeys {
			if _, set := previousObject[key]; !set {
				delete(object, key)
			}
		}
	}
	merged := mergeRedacted(api, previous)
	if jsonEqual(merged, previous) {
		return prior
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return raw
	}
	return types.StringValue(string(encoded))
}

// misplacedPlaceholders lists the paths ("url", "headers.Authorization",
// "urls[1]") where config holds the redacted placeholder but the prior state
// doesn't hold the same placeholder at the same path. Sending one there would
// pass the placeholder off as a real value; one carried over unchanged from
// state (an import below the admin tier) keeps the stored secret. For a
// create, pass a null prior: every placeholder is misplaced. Invalid JSON
// yields nothing — it is reported separately.
func misplacedPlaceholders(config string, prior types.String) []string {
	planned, err := decodeConfig(config)
	if err != nil {
		return nil
	}
	var previous interface{}
	if !prior.IsNull() && !prior.IsUnknown() {
		previous, _ = decodeConfig(prior.ValueString())
	}
	var paths []string
	collectMisplaced(planned, previous, "", &paths)
	sort.Strings(paths)
	return paths
}

func collectMisplaced(planned, previous interface{}, path string, paths *[]string) {
	switch p := planned.(type) {
	case string:
		if p != redactedSecret {
			return
		}
		if s, ok := previous.(string); !ok || s != redactedSecret {
			if path == "" {
				path = "(the whole config)"
			}
			*paths = append(*paths, path)
		}
	case map[string]interface{}:
		prev, _ := previous.(map[string]interface{})
		for key, value := range p {
			child := key
			if path != "" {
				child = path + "." + key
			}
			collectMisplaced(value, prev[key], child, paths)
		}
	case []interface{}:
		prev, _ := previous.([]interface{})
		for i, value := range p {
			var before interface{}
			if i < len(prev) {
				before = prev[i]
			}
			collectMisplaced(value, before, fmt.Sprintf("%s[%d]", path, i), paths)
		}
	}
}

// placeholderErrorDetail explains a misplacedPlaceholders result without
// echoing any config value (config is sensitive).
func placeholderErrorDetail(paths []string) string {
	return fmt.Sprintf(
		"config holds the %q placeholder at %s. Flaggr shows it in place of secrets to tokens below the admin tier; it is not a real value. "+
			"Set the secret itself (for example from a sensitive variable). A placeholder is only sent when the same placeholder is already in state at that path, "+
			"which keeps the secret Flaggr has stored.",
		redactedSecret, strings.Join(paths, ", "),
	)
}
