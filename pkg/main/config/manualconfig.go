package config

import (
	"reflect"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Keys of ManualConfig that [global] can default.
const (
	ManualKeyIdentifiedby = "identifiedby"
	manualKeyDontUpgrade  = "dont_upgrade"
	manualKeyDontSearch   = "dont_search"
)

// ManualKeys is a set of ManualConfig's TOML keys, one bit per field: a
// map per entry made decoding a series file allocate for every key of
// every entry. Keys that aren't ManualConfig fields are not recorded.
type ManualKeys uint64

// manualKeyBits maps each TOML key of ManualConfig to its bit.
var manualKeyBits = func() map[string]ManualKeys {
	t := reflect.TypeFor[ManualConfig]()
	bits := make(map[string]ManualKeys, t.NumField())

	for i := range t.NumField() {
		key, _, _ := strings.Cut(t.Field(i).Tag.Get("toml"), ",")
		if key == "" || key == "-" {
			continue
		}

		if len(bits) == 64 {
			panic("config: ManualConfig has more TOML keys than ManualKeys can hold")
		}

		bits[key] = 1 << len(bits)
	}

	return bits
}()

// Has reports whether key is in the set.
func (k ManualKeys) Has(key string) bool {
	return k&manualKeyBits[key] != 0
}

// Set adds key to the set; a key that isn't a ManualConfig field is ignored.
func (k *ManualKeys) Set(key string) {
	*k |= manualKeyBits[key]
}

// DecodeManualConfig decodes a manual config file (series, audiobook, music,
// book entries). It records which keys each entry sets (ManualConfig.Explicit)
// and fills unset identifiedby, dont_upgrade and dont_search from the file's
// [global] section.
func DecodeManualConfig(data []byte) (*MainManualConfig, error) {
	explicit, ok := scanManualKeys(data)

	// Sized up front: the decoder appends entry by entry, and regrowing a
	// slice of these large structs was most of the decode's memory.
	cfg := MainManualConfig{Config: make([]ManualConfig, 0, len(explicit))}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if !ok || len(explicit) != len(cfg.Config) {
		// Entries the scan doesn't follow (an inline `config = [{...}]`
		// array): read the keys from a generic decode instead.
		var err error
		if explicit, err = decodeManualKeys(data); err != nil {
			return nil, err
		}
	}

	for i := range cfg.Config {
		e := &cfg.Config[i]
		if i < len(explicit) {
			e.Explicit = explicit[i]
		}

		cfg.Global.apply(e)
	}

	return &cfg, nil
}

// scanManualKeys lists the keys each [[config]] entry sets by walking the
// TOML expressions, without building their values. ok is false for a
// document it can't read; Unmarshal has already accepted it, so the caller
// falls back to decodeManualKeys.
func scanManualKeys(data []byte) (keys []ManualKeys, ok bool) {
	var (
		p       unstable.Parser
		inEntry bool // key/values belong to the last [[config]] entry
	)

	p.Reset(data)

	for p.NextExpression() {
		e := p.Expression()

		switch e.Kind {
		case unstable.KeyValue:
			if inEntry {
				it := e.Key()
				if it.Next() {
					keys[len(keys)-1] |= manualKeyBits[string(it.Node().Data)]
				}
			}
		case unstable.ArrayTable, unstable.Table:
			it := e.Key()
			if !it.Next() || string(it.Node().Data) != "config" {
				inEntry = false
				continue
			}

			if !it.Next() {
				// [[config]] starts an entry; a [config] table isn't one.
				inEntry = e.Kind == unstable.ArrayTable
				if inEntry {
					keys = append(keys, 0)
				}

				continue
			}

			// [config.sub] or [[config.sub]]: sets key sub of the entry.
			if len(keys) > 0 {
				keys[len(keys)-1] |= manualKeyBits[string(it.Node().Data)]
			}

			inEntry = false
		}
	}

	return keys, p.Error() == nil
}

// decodeManualKeys lists the keys of each [[config]] entry from a generic
// decode; the slow path for documents scanManualKeys can't follow.
func decodeManualKeys(data []byte) ([]ManualKeys, error) {
	var raw struct {
		Config []map[string]any `toml:"config"`
	}

	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	keys := make([]ManualKeys, len(raw.Config))
	for i, entry := range raw.Config {
		for k := range entry {
			keys[i].Set(k)
		}
	}

	return keys, nil
}

// apply fills the fields e doesn't set from the defaults.
func (g *ManualGlobalConfig) apply(e *ManualConfig) {
	if !e.Explicit.Has(ManualKeyIdentifiedby) && g.Identifiedby != "" {
		e.Identifiedby = g.Identifiedby
	}

	if !e.Explicit.Has(manualKeyDontUpgrade) && g.Upgrade != nil {
		e.DontUpgrade = !*g.Upgrade
	}

	if !e.Explicit.Has(manualKeyDontSearch) && g.Search != nil {
		e.DontSearch = !*g.Search
	}
}

// DefaultFor returns the value [global] gives the key, and whether it gives one.
func (g *ManualGlobalConfig) DefaultFor(key string) (any, bool) {
	switch key {
	case ManualKeyIdentifiedby:
		return g.Identifiedby, g.Identifiedby != ""
	case manualKeyDontUpgrade:
		if g.Upgrade != nil {
			return !*g.Upgrade, true
		}
	case manualKeyDontSearch:
		if g.Search != nil {
			return !*g.Search, true
		}
	}

	return nil, false
}
