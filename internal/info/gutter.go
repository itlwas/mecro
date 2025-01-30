package info
type GutterMessage struct {
	lineNum int
	msg     string
	kind    int
}
const (
	GutterInfo = iota
	GutterWarning
	GutterError
)