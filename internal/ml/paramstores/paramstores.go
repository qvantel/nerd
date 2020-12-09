package paramstores

import (
	"errors"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
)

// NetParams serves to ensure that any new net implementation will come with instructions for writing and reading it
// to and from the store as well as a way to get a standard representation of it that the API can use
type NetParams interface {
	Brief() *types.BriefNet
	Marshal() ([]byte, error)
	Unmarshal(b []byte) error
}

// NetParamStore is an abstraction over the storage service that will be used to share the nets between instances, it
// should allow queries by ID and be fairly performant (Redis is a good option here but it's good to have flexibility)
type NetParamStore interface {
	Delete(id string) error
	// Returns an array of net IDs
	List(offset, limit int) ([]string, int, error)
	// Load retrieves the NetParams for a net from a store, will return true and a nil error if found
	Load(id string, np NetParams) (bool, error)
	Save(id string, np NetParams) error
}

// New creates and returns the corresponding type of param store for the given configuration
func New(conf config.Config) (NetParamStore, error) {
	switch conf.ML.StoreType {
	case "file":
		return NewFileAdapter(conf.ML.StoreParams)
	case "redis":
		return NewRedisAdapter(conf.ML.StoreParams)
	default:
		return nil, errors.New(conf.ML.StoreType + " is not a valid net param store type")
	}
}
