package paramstores

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/qvantel/nerd/internal/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testRedisStore NetParamStore

func getTestStore(url string) (NetParamStore, error) {
	var storeParams map[string]interface{}
	json.Unmarshal([]byte(`{"URL": "`+url+`"}`), &storeParams)
	conf := config.Config{
		ML: config.MLParams{
			StoreType:   config.RedisParamStore,
			StoreParams: storeParams,
		},
	}
	return New(conf)
}

func startRedis(ctx context.Context) (redis testcontainers.Container, url string, err error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:6.0.10-alpine3.13",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redis, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}
	endpoint, err := redis.Endpoint(ctx, "")
	if err != nil {
		return nil, "", err
	}
	return redis, endpoint, nil
}

func TestList(t *testing.T) {
	id, err := initTest(testRedisStore)
	if err != nil {
		t.Fatalf("Failed to initialize net param store (%s)", err.Error())
	}
	defer testRedisStore.Delete(id)

	var res []string
	cursor := 0
	ids := []string{}
	for {
		res, cursor, err = testRedisStore.List(cursor, 10, "*")
		ids = append(ids, res...)
		if cursor == 0 {
			break
		}
	}

	if len(ids) != 1 {
		t.Fatalf("Expected List to return one ID, got %d instead", len(ids))
	}
	if ids[0] != id {
		t.Fatalf("Expected List to return ID %s, got %s instead", id, ids[0])
	}
}

func TestMain(m *testing.M) {
	// Setup
	ctx := context.Background()
	redis, url, err := startRedis(ctx)
	if err != nil {
		fmt.Printf("Error starting test Redis container (%s)", err.Error())
		os.Exit(1)
	}
	testRedisStore, err = getTestStore(url)
	if err != nil {
		fmt.Printf("Failed to get net param store (%s)", err.Error())
		os.Exit(1)
	}
	// Run
	code := m.Run()
	// Teardown
	if err == nil {
		redis.Terminate(ctx)
	}
	os.Exit(code)
}
