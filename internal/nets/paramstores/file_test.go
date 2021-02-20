package paramstores

import (
	"encoding/json"
	"testing"

	"github.com/qvantel/nerd/internal/config"
)

func getTestFileStore() (NetParamStore, error) {
	var storeParams map[string]interface{}
	json.Unmarshal([]byte(`{"Path": "."}`), &storeParams)
	conf := config.Config{
		ML: config.MLParams{
			StoreType:   config.FileParamStore,
			StoreParams: storeParams,
		},
	}
	return New(conf)
}

func TestListNetFiles(t *testing.T) {
	nps, err := getTestFileStore()
	if err != nil {
		t.Fatalf("Failed to get net param store (%s)", err.Error())
	}
	id, err := initTest(nps)
	if err != nil {
		t.Fatalf("Failed to initialize net param store (%s)", err.Error())
	}
	defer nps.Delete(id)

	var res []string
	cursor := 0
	ids := []string{}
	for {
		res, cursor, err = nps.List(cursor, 10, "*")
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

func TestLoadNetFile(t *testing.T) {
	nps, err := getTestFileStore()
	if err != nil {
		t.Fatalf("Failed to get net param store (%s)", err.Error())
	}
	id, err := initTest(nps)
	if err != nil {
		t.Fatalf("Failed to initialize net param store (%s)", err.Error())
	}
	defer nps.Delete(id)

	var params MLPParams
	found, err := nps.Load("does-not-exist-mlp", &params)
	if err != nil {
		t.Fatalf("Failed to load net params from store (%s)", err.Error())
	}
	if found {
		t.Error("Found non-existent net")
	}

	found, err = nps.Load(id, &params)
	if err != nil {
		t.Fatalf("Failed to load net params from store (%s)", err.Error())
	}
	if !found {
		t.Fatal("Failed to find existing net")
	}
	if params.LearningRate != 0.25 {
		t.Errorf("Incorrect learning rate for retrieved params, expected 0.25, got %f", params.LearningRate)
	}
}
