package cache

import (
	"flag"
	"sync"
	"time"

	"github.com/cortexproject/cortex/pkg/util"
	"github.com/go-kit/kit/log/level"
	"github.com/gomodule/redigo/redis"
)

// RedisClient interface exists for mocking redisClient.
type RedisClient interface {
	Set(string, []byte) error
	Get(string) ([]byte, error)
	Stop()
}

// redisClient is a redis client that gets its server list from SRV
// records, and periodically updates that ServerList.
type redisClient struct {
	hostname string
	service  string

	pool *redis.Pool
	//conn *redis.ConnWithTimeout
	conn redis.Conn

	quit chan struct{}
	wait sync.WaitGroup
}

// RedisClientConfig defines how a RedisClient should be constructed.
type RedisClientConfig struct {
	Host           string        `yaml:"host,omitempty"`
	Timeout        time.Duration `yaml:"timeout,omitempty"`
	MaxIdleConns   int           `yaml:"max_idle_conns,omitempty"`
	MaxActiveConns int           `yaml:"max_active_conns,omitempty"`
	UpdateInterval time.Duration `yaml:"update_interval,omitempty"`
}

// RegisterFlagsWithPrefix adds the flags required to config this to the given FlagSet
func (cfg *RedisClientConfig) RegisterFlagsWithPrefix(prefix, description string, f *flag.FlagSet) {
	f.StringVar(&cfg.Host, prefix+"redis.hostname", "", description+"Hostname for redis service to use when caching chunks. If empty, no redis will be used.")
	f.DurationVar(&cfg.Timeout, prefix+"redis.timeout", 100*time.Millisecond, description+"Maximum time to wait before giving up on redis requests.")
	f.IntVar(&cfg.MaxIdleConns, prefix+"redis.max-idle-conns", 80, description+"Maximum number of idle connections in pool.")
	f.IntVar(&cfg.MaxActiveConns, prefix+"redis.max-active-conns", 12000, description+"Maximum number of active connections in pool.")
	f.DurationVar(&cfg.UpdateInterval, prefix+"redis.update-interval", 1*time.Minute, description+"Period with which to ping redis service.")
}

// NewRedisClient creates a new RedisClient
func NewRedisClient(cfg RedisClientConfig) RedisClient {
	pool := &redis.Pool{
		MaxIdle:   cfg.MaxIdleConns,
		MaxActive: cfg.MaxActiveConns,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", cfg.Host)
			if err != nil {
				return nil, err
			}
			return c, err
		},
	}

	client := &redisClient{
		hostname: cfg.Host,
		pool:     pool,
		conn:     pool.Get(),
	}

	if err := client.ping(); err != nil {
		level.Error(util.Logger).Log("msg", "error connecting to redis", "host", cfg.Host, "err", err)
	}

	client.wait.Add(1)
	go client.updateLoop(cfg.UpdateInterval)
	return client
}

func (c *redisClient) Set(key string, val []byte) error {
	_, err := c.conn.Do("SET", key, val)
	return err
}

func (c *redisClient) Get(key string) ([]byte, error) {
	return redis.Bytes(c.conn.Do("GET", key))
}

// Stop the redis client.
func (c *redisClient) Stop() {
	close(c.quit)
	c.wait.Wait()
}

func (c *redisClient) updateLoop(updateInterval time.Duration) error {
	defer c.wait.Done()
	ticker := time.NewTicker(updateInterval)
	var err error
	for {
		select {
		case <-ticker.C:
			err = c.ping()
			if err != nil {
				level.Warn(util.Logger).Log("msg", "error connecting to redis", "err", err)
				c.conn = c.pool.Get()
			}
		case <-c.quit:
			ticker.Stop()
		}
	}
}

func (c *redisClient) ping() error {
	pong, err := c.conn.Do("PING")
	if err != nil {
		return err
	}
	s, err := redis.String(pong, err)
	if err != nil {
		return err
	}
	return nil
}
