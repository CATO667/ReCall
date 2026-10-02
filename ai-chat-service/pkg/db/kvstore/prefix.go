package kvstore

import (
	"ai-chat-service/pkg/config"
	"strings"
)

func ServicePrefix() string {
	cnf := config.GetConfig()
	if cnf != nil && cnf.KVStore.Prefix != "" {
		return cnf.KVStore.Prefix
	}
	return "ai_chat_service_"
}

func Key(parts ...string) string {
	key := ServicePrefix() + strings.Join(parts, "_")
	return key
}

// QAKey is deprecated; new QA records use QABodyKey + QAMetaKey.
func QAKey(hash string) string {
	return Key("qa", hash)
}

// QABodyKey returns the raw question as the KV key (value = answer only).
func QABodyKey(question string) string {
	return question
}

func LegacyQABodyKey(question string) string {
	return Key("qa", "body", question)
}

func QAMetaKey(hash string) string {
	return Key("qa", "meta", hash)
}

// ParseQAHashFromKey extracts the QA content hash from a kvstore QA key.
func ParseQAHashFromKey(qaKey string) string {
	prefix := ServicePrefix()
	if !strings.HasPrefix(qaKey, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(qaKey, prefix)
	if strings.HasPrefix(rest, "qa_meta_") {
		return strings.TrimPrefix(rest, "qa_meta_")
	}
	if strings.HasPrefix(rest, "qa_") && !strings.HasPrefix(rest, "qa_body_") &&
		rest != "qa_index" && !strings.HasPrefix(rest, "qa_topic_") {
		return strings.TrimPrefix(rest, "qa_")
	}
	return ""
}

func QAIndexKey() string {
	return Key("qa", "index")
}

func QATopicIndexKey() string {
	return Key("qa", "topic", "index")
}

func QATopicKey(topicID string) string {
	return Key("qa", "topic", topicID)
}

func QATopicQAKey(topicID string) string {
	return Key("qa", "topic", topicID, "qas")
}

func DocHashKey(contentHash string) string {
	return Key("doc", "hash", contentHash)
}

func CtxKey(messageID string) string {
	return Key("ctx", messageID)
}

func VecKey(qaKey string) string {
	return Key("vec", qaKey)
}

func DocIndexKey() string {
	return Key("doc", "index")
}

func DocMetaKey(docID string) string {
	return Key("doc", "meta", docID)
}

func DocChunkIndexKey(docID string) string {
	return Key("doc", "chunks", docID)
}

func ChunkKey(chunkID string) string {
	return Key("chunk", chunkID)
}
