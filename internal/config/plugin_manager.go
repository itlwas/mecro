package config
import (
	"bytes"
	"encoding/json"
	"errors"
)
var (
	ErrMissingName = errors.New("Missing or empty name field")
	ErrMissingDesc = errors.New("Missing or empty description field")
	ErrMissingSite = errors.New("Missing or empty website field")
)
type PluginInfo struct {
	Name string `json:"Name"`
	Desc string `json:"Description"`
	Site string `json:"Website"`
}
func NewPluginInfo(data []byte) (*PluginInfo, error) {
	var info []PluginInfo
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}
	return &info[0], nil
}