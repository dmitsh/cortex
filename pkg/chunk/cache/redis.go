package cache

import (
	"context"
	"flag"

	"github.com/cortexproject/cortex/pkg/util"
	"github.com/go-kit/kit/log/level"
	"github.com/gomodule/redigo/redis"
)

// RedisConfig is config to make a Memcached
type RedisConfig struct {
}

// RegisterFlagsWithPrefix adds the flags required to config this to the given FlagSet
func (cfg *RedisConfig) RegisterFlagsWithPrefix(prefix, description string, f *flag.FlagSet) {

}

// Redis type caches chunks in redis
type Redis struct {
	cfg   RedisConfig
	redis RedisClient
	name  string
}

func NewRedis(cfg RedisConfig, client RedisClient, name string) *Redis {
	c := &Redis{
		cfg:   cfg,
		redis: client,
		name:  name,
	}
	return c
}

// Fetch gets keys from the cache. The keys that are found must be in the order of the keys requested.
func (c *Redis) Fetch(ctx context.Context, keys []string) (found []string, bufs [][]byte, missed []string) {
	for _, key := range keys {

		if buf, err := c.redis.Get(key); err == nil {
			found = append(found, key)
			bufs = append(bufs, buf)
		} else {
			missed = append(missed, key)
			if err != redis.ErrNil {
				level.Error(util.Logger).Log("msg", "failed to get from redis", "name", c.name, "err", err)
			}
		}
	}
	return
}

// Store stores the key in the cache.
func (c *Redis) Store(ctx context.Context, keys []string, bufs [][]byte) {
	for i, key := range keys {
		if err := c.redis.Set(key, bufs[i]); err != nil {
			level.Error(util.Logger).Log("msg", "failed to put to redis", "name", c.name, "err", err)
		}
	}
}

// Stop does nothing.
func (c *Redis) Stop() error {
	c.redis.Stop()
	return nil
}
