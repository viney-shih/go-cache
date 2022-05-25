package cache

var (
	evictTopic = customKey(topicDelim, packageKey, topicKey, "evict")
)

type evictEvent struct {
	ID   string
	Keys []string
}
