package cache

import "strings"

const (
	packageKey = "ca"
	topicKey   = "tp"

	// delimiters
	cacheDelim = ":"
	topicDelim = "#"
)

func customKey(delimiter string, components ...string) string {
	return strings.Join(components, delimiter)
}

func getCacheKey(pfx, key string) string {
	return customKey(cacheDelim, packageKey, pfx, key)
}

func getCacheKeys(pfx string, keys []string) []string {
	cacheKeys := make([]string, len(keys))
	for i, k := range keys {
		cacheKeys[i] = getCacheKey(pfx, k)
	}

	return cacheKeys
}

func getPrefixAndKey(cacheKey string) (string, string) {
	// cacheKey = packageKey + prefix + key
	idx := strings.Index(cacheKey, cacheDelim)
	if idx < 0 {
		return cacheKey, ""
	}

	// mixedKey = prefix + key
	mixedKey := cacheKey[idx+len(cacheDelim):]
	idx = strings.Index(mixedKey, cacheDelim)
	if idx < 0 {
		return mixedKey, ""
	}

	return mixedKey[:idx], mixedKey[idx+len(cacheDelim):]
}
