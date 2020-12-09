package paramstores

import (
	"io/ioutil"
	"os"
	"strings"
)

// FileAdapter is a neural net param store implementation that uses the filesystem, its main purpose is to facilitate
// testing. Given its low performance it is strongly discouraged for production use
type FileAdapter struct {
	Path string
}

// NewFileAdapter returns an initialized file net param store object
func NewFileAdapter(conf map[string]interface{}) (*FileAdapter, error) {
	return &FileAdapter{Path: conf["Path"].(string)}, nil
}

// Delete can be used to delete the state of a specific neural net from a file on disk
func (fa FileAdapter) Delete(id string) error {
	err := os.Remove(fa.Path + "/" + id)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

// List can be used to get the IDs of the stored nets
func (fa FileAdapter) List(offset, limit int) ([]string, int, error) {
	files, err := ioutil.ReadDir(fa.Path)
	if err != nil {
		return nil, 0, err
	}
	var ids []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		parts := strings.Split(file.Name(), "-")
		if len(parts) < 4 {
			continue
		}
		ids = append(ids, file.Name())
	}
	return ids, 0, nil
}

// Load can be used to retrieve the state of a specific neural net from a file on disk
func (fa FileAdapter) Load(id string, np NetParams) (bool, error) {
	value, err := ioutil.ReadFile(fa.Path + "/" + id)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	err = np.Unmarshal([]byte(value))
	return true, err
}

// Save can be used to upsert the state of a specific neural net to a file on disk
func (fa FileAdapter) Save(id string, np NetParams) error {
	value, err := np.Marshal()
	if err != nil {
		return err
	}
	f, err := os.Create(fa.Path + "/" + id)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(string(value))
	if err != nil {
		return err
	}
	f.Sync()
	return nil
}
