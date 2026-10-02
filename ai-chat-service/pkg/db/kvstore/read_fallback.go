package kvstore

import (
	"context"
	"log"
	"sync/atomic"
)

var readFallbackLogged atomic.Bool

func GetWithReadFallback(ctx context.Context, key string) ([]byte, error) {
	readPool := GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)
	data, err := rc.Get(ctx, key)
	if err != nil || data != nil || !readWriteSplit {
		return data, err
	}
	writePool := GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	data, err = wc.Get(ctx, key)
	if err == nil && data != nil && readFallbackLogged.CompareAndSwap(false, true) {
		log.Printf("kvstore: read pool miss, served from write pool (check read replica sync)")
	}
	return data, err
}

func commandReadWithFallback(ctx context.Context, args ...string) (*Reply, error) {
	readPool := GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)
	reply, err := rc.Command(ctx, args...)
	if err != nil || !readWriteSplit || !shouldFallbackReadReply(args, reply) {
		return reply, err
	}
	writePool := GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	fallback, err := wc.Command(ctx, args...)
	if err == nil && shouldFallbackReadReply(args, reply) && !shouldFallbackReadReply(args, fallback) {
		if readFallbackLogged.CompareAndSwap(false, true) {
			log.Printf("kvstore: read pool empty for %s, served from write pool (check read replica sync)", args[0])
		}
		return fallback, err
	}
	return reply, err
}

func shouldFallbackReadReply(args []string, reply *Reply) bool {
	if reply == nil {
		return true
	}
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "VCOUNT":
		return reply.Type == ':' && reply.Integer == 0
	case "VSEARCH":
		return reply.Type == '*' && len(reply.Array) == 0
	case "GET":
		return reply.IsNil
	default:
		return false
	}
}
