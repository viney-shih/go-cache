package cache

import (
	"strings"
	"sync"
)

const (
	packageKey = "ca"
	topicKey   = "tp"

	// delimiters
	cacheDelim = ":"
	topicDelim = "#"
)

var (
	regPkgKey = packageKey
	// regKeyOnce limits key registeration happening once
	regKeyOnce = sync.Once{}
)

func registerKey(pkgKey string) {
	regKeyOnce.Do(func() {
		regPkgKey = pkgKey
	})
}

func customKey(delimiter string, components ...string) string {
	return strings.Join(components, delimiter)
}

func getTopic(topic string) string {
	return customKey(topicDelim, regPkgKey, topicKey, topic)
}

func getCacheKey(pfx, key string) string {
	if regPkgKey == "" {
		return customKey(cacheDelim, pfx, key)
	}

	return customKey(cacheDelim, regPkgKey, pfx, key)
}

func getCacheKeys(pfx string, keys []string) []string {
	cacheKeys := make([]string, len(keys))
	for i, k := range keys {
		cacheKeys[i] = getCacheKey(pfx, k)
	}

	return cacheKeys
}

func getPrefixAndKey(cacheKey string) (string, string) {
	// 1) cacheKey = regPkgKey + prefix + key (normal case)
	// 2) cacheKey = prefix + key (if customized package key is empty)
	idx := strings.Index(cacheKey, cacheDelim)
	if idx < 0 {
		return cacheKey, "" // should not happen
	}

	if regPkgKey == "" {
		return cacheKey[:idx], cacheKey[idx+len(cacheDelim):]
	}

	// mixedKey = prefix + key
	mixedKey := cacheKey[idx+len(cacheDelim):]
	idx = strings.Index(mixedKey, cacheDelim)
	if idx < 0 {
		return mixedKey, ""
	}

	return mixedKey[:idx], mixedKey[idx+len(cacheDelim):]
}
