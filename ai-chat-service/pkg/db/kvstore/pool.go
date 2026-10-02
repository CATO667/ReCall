package kvstore

import (
	"ai-chat-service/pkg/config"
	"log"
	"sync"
)

type Pool interface {
	Get() *Client
	Put(client *Client)
}

type pool struct {
	host string
	port int
	pool sync.Pool
}

func (p *pool) Get() *Client {
	client, ok := p.pool.Get().(*Client)
	if ok && client != nil {
		if err := client.ping(); err == nil {
			return client
		}
		_ = client.Close()
	}
	c, err := NewClient(p.host, p.port)
	if err != nil {
		panic(err)
	}
	return c
}

func (p *pool) Put(client *Client) {
	if client == nil {
		return
	}
	if err := client.ping(); err != nil {
		_ = client.Close()
		return
	}
	p.pool.Put(client)
}

var (
	writePool      Pool
	readPool       Pool
	readWriteSplit bool
)

func InitPools(cnf *config.Config) {
	writeHost := cnf.KVStore.WriteHost
	if writeHost == "" {
		writeHost = "127.0.0.1"
	}
	writePort := cnf.KVStore.Port
	if writePort == 0 {
		writePort = 2000
	}
	readHost := cnf.KVStore.ReadHost
	if readHost == "" {
		readHost = writeHost
	}
	readPort := cnf.KVStore.ReadPort
	if readPort == 0 {
		readPort = writePort
	}
	writePool = newPool(writeHost, writePort)
	readPool = newPool(readHost, readPort)
	readWriteSplit = readHost != writeHost || readPort != writePort
	log.Printf("kvstore pools: write=%s:%d read=%s:%d split=%v", writeHost, writePort, readHost, readPort, readWriteSplit)
	if _, err := NewClient(writeHost, writePort); err != nil {
		panic("kvstore write pool: " + err.Error())
	}
	if readHost != writeHost || readPort != writePort {
		if _, err := NewClient(readHost, readPort); err != nil {
			panic("kvstore read pool: " + err.Error())
		}
	}
}

func newPool(host string, port int) Pool {
	return &pool{host: host, port: port}
}

func GetWritePool() Pool {
	return writePool
}

func GetReadPool() Pool {
	return readPool
}
