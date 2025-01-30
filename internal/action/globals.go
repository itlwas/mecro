package action
import "github.com/zyedidia/micro/v2/internal/buffer"
var InfoBar *InfoPane
var LogBufPane *BufPane
func InitGlobals() {
	InfoBar = NewInfoBar()
	buffer.LogBuf = buffer.NewBufferFromString("", "Log", buffer.BTLog)
}
func GetInfoBar() *InfoPane {
	return InfoBar
}
func WriteLog(s string) {
	buffer.WriteLog(s)
	if LogBufPane != nil {
		LogBufPane.CursorEnd()
	}
}
func (h *BufPane) OpenLogBuf() {
	LogBufPane = h.HSplitBuf(buffer.LogBuf)
	LogBufPane.CursorEnd()
}