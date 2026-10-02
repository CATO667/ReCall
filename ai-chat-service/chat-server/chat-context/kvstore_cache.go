package chat_context

import (
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"encoding/json"
)

type kvstoreCache struct{}

func NewKVStoreCache() ContextCache {
	return &kvstoreCache{}
}

func (c *kvstoreCache) Get(key string) (*ChatMessage, error) {
	key = kvstore.CtxKey(key)
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	str, err := client.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	if str == nil {
		return nil, nil
	}
	value := &ChatMessage{}
	err = json.Unmarshal(str, value)
	return value, err
}

func (c *kvstoreCache) Set(key string, value *ChatMessage, ttl int) error {
	key = kvstore.CtxKey(key)
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	pool := kvstore.GetWritePool()
	client := pool.Get()
	defer pool.Put(client)
	ctx := context.Background()
	if ttl <= 0 {
		return client.SetOrMod(ctx, key, string(bytes))
	}
	return client.SetWithExpire(ctx, key, string(bytes), ttl)
}

func (c *kvstoreCache) Close() {}
