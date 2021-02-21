package paramstores

import (
	"bufio"
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"

	redis "github.com/mediocregopher/radix/v3"
	"github.com/mediocregopher/radix/v3/resp/resp2"
	"github.com/qvantel/nerd/internal/logger"
)

// RedisAdapter is the neural net param store implementation for Redis
type RedisAdapter struct {
	client   redis.Client
	sentinel *redis.Sentinel
}

// readClient returns a client for a random Redis instance, as reads can be handled by secondary replicas too
func (ra *RedisAdapter) readClient() (redis.Client, error) {
	if ra.sentinel == nil {
		return ra.client, nil
	}
	primary, secondaries := ra.sentinel.Addrs()
	secondaries = append(secondaries, primary)
	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(len(secondaries))
	client, err := ra.sentinel.Client(secondaries[i])
	return client, err
}

// writeClient returns a client for the primary Redis instance, as that's the only one that can handle writes
func (ra *RedisAdapter) writeClient() (redis.Client, error) {
	if ra.sentinel == nil {
		return ra.client, nil
	}
	primary, _ := ra.sentinel.Addrs()
	client, err := ra.sentinel.Client(primary)
	return client, err
}

// NewRedisAdapter returns an initialized Redis param store object
func NewRedisAdapter(conf map[string]interface{}) (*RedisAdapter, error) {
	group, found := conf["group"]
	if found {
		sentinel, err := redis.NewSentinel(group.(string), strings.Split(conf["URLs"].(string), ","))
		return &RedisAdapter{sentinel: sentinel}, err
	}
	pool, err := redis.NewPool("tcp", conf["URL"].(string), 10)
	return &RedisAdapter{client: pool}, err
}

// Delete can be used to delete the state of a specific neural net from Redis
func (ra *RedisAdapter) Delete(id string) error {
	var (
		value    int
		redisErr resp2.Error
	)
	client, err := ra.writeClient()
	if err != nil {
		return err
	}
	err = client.Do(redis.Cmd(&value, "DEL", id))
	if errors.As(err, &redisErr) {
		logger.Error("Redis error returned while deletening a net", redisErr.E)
		return redisErr.E
	}
	return err
}

type scanResult struct {
	cur  int
	keys []string
}

// UnmarshalRESP is based on the private method with the same name in the radix library and needed here because said
// library doesn't provide a good interface for decoupled iteration (where the client of the API needs to know what the
// value of the cursor is) which means that we have to use the plain Cmd approach and parse the result ourselves
func (s *scanResult) UnmarshalRESP(br *bufio.Reader) error {
	var ah resp2.ArrayHeader
	err := ah.UnmarshalRESP(br)
	if err != nil {
		return err
	} else if ah.N != 2 {
		return errors.New("not enough parts returned")
	}

	var c resp2.BulkString
	if err := c.UnmarshalRESP(br); err != nil {
		return err
	}

	s.cur, err = strconv.Atoi(c.S)
	if err != nil {
		logger.Error("Error trying to convert cursor to int (raw: "+c.S+")", err)
		return err
	}
	s.keys = s.keys[:0]

	return (resp2.Any{I: &s.keys}).UnmarshalRESP(br)
}

// List can be used to page through the IDs of the nets that are stored in Redis by starting with offset 0 and then
// continuing to call the method with the updated cursor value it returns until it becomes 0
func (ra *RedisAdapter) List(offset, limit int, pattern string) ([]string, int, error) {
	var (
		res      scanResult
		redisErr resp2.Error
	)
	client, err := ra.readClient()
	if err != nil {
		return nil, 0, err
	}
	args := []string{strconv.Itoa(offset), "COUNT", strconv.Itoa(limit)}
	if pattern != "*" {
		args = append(args, "MATCH")
		args = append(args, pattern)
	}
	err = client.Do(redis.Cmd(&res, "SCAN", args...))
	if errors.As(err, &redisErr) {
		logger.Error("Redis error returned while listing nets", redisErr.E)
		return nil, 0, redisErr.E
	}

	return res.keys, res.cur, nil
}

// Load can be used to retrieve the state of a specific neural net from Redis
func (ra *RedisAdapter) Load(id string, np NetParams) (bool, error) {
	var (
		value    string
		redisErr resp2.Error
	)
	client, err := ra.readClient()
	if err != nil {
		return false, err
	}
	err = client.Do(redis.Cmd(&value, "GET", id))
	if errors.As(err, &redisErr) {
		logger.Error("Redis error returned while loading a net", redisErr.E)
		return false, redisErr.E
	}
	if value == "" {
		return false, err
	}
	err = np.Unmarshal([]byte(value))
	return true, err
}

// Save can be used to upsert the state of a specific neural net to Redis
func (ra *RedisAdapter) Save(id string, np NetParams) error {
	value, err := np.Marshal()
	if err != nil {
		return err
	}
	client, err := ra.writeClient()
	if err != nil {
		return err
	}
	return client.Do(redis.Cmd(nil, "SET", id, string(value)))
}
