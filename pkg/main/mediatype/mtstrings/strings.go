// Package mtstrings provides the GetStringsMap function for retrieving
// media-type-specific strings. This package has no imports to avoid circular
// dependencies - media type packages register their string maps during init().
package mtstrings

// registry holds registered string maps by media type.
var registry = make(map[uint]map[string]string)

// Register adds a string map for the specified media type.
// This should be called from init() in each media type package.
func Register(mediaType uint, stringsMap map[string]string) {
	registry[mediaType] = stringsMap
}

// GetStringsMap returns a type-specific string for the given key and media type.
// Returns empty string if key is not found for the given type.
func GetStringsMap(isType uint, key string) string {
	if m, ok := registry[isType]; ok {
		return m[key]
	}

	return ""
}
