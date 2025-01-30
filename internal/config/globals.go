package config
const (
	DoubleClickThreshold = 400
)
var Bindings map[string]map[string]string
func init() {
	Bindings = map[string]map[string]string{
		"command":  make(map[string]string),
		"buffer":   make(map[string]string),
		"terminal": make(map[string]string),
	}
}