package info
import (
	"fmt"
	"github.com/zyedidia/micro/v2/internal/buffer"
)
type InfoBuf struct {
	*buffer.Buffer
	HasPrompt  bool
	HasMessage bool
	HasError   bool
	HasYN      bool
	PromptType string
	Msg    string
	YNResp bool
	History    map[string][]string
	HistoryNum int
	HistorySearch       bool
	HistorySearchPrefix string
	HasGutter bool
	PromptCallback func(resp string, canceled bool)
	EventCallback  func(resp string)
	YNCallback     func(yes bool, canceled bool)
}
func NewBuffer() *InfoBuf {
	ib := new(InfoBuf)
	ib.History = make(map[string][]string)
	ib.Buffer = buffer.NewBufferFromString("", "", buffer.BTInfo)
	ib.LoadHistory()
	return ib
}
func (i *InfoBuf) Close() {
	i.SaveHistory()
}
func (i *InfoBuf) Message(msg ...interface{}) {
	if !i.HasPrompt {
		displayMessage := fmt.Sprint(msg...)
		i.Msg = displayMessage
		i.HasMessage, i.HasError = true, false
	}
}
func (i *InfoBuf) GutterMessage(msg ...interface{}) {
	i.Message(msg...)
	i.HasGutter = true
}
func (i *InfoBuf) ClearGutter() {
	i.HasGutter = false
	i.Message("")
}
func (i *InfoBuf) Error(msg ...interface{}) {
	if !i.HasPrompt {
		i.Msg = fmt.Sprint(msg...)
		i.HasMessage, i.HasError = false, true
	}
}
func (i *InfoBuf) Prompt(prompt string, msg string, ptype string, eventcb func(string), donecb func(string, bool)) {
	if i.HasPrompt {
		i.DonePrompt(true)
	}
	if _, ok := i.History[ptype]; !ok {
		i.History[ptype] = []string{""}
	} else {
		i.History[ptype] = append(i.History[ptype], "")
	}
	i.HistoryNum = len(i.History[ptype]) - 1
	i.HistorySearch = false
	i.PromptType = ptype
	i.Msg = prompt
	i.HasPrompt = true
	i.HasMessage, i.HasError, i.HasYN = false, false, false
	i.HasGutter = false
	i.PromptCallback = donecb
	i.EventCallback = eventcb
	i.Buffer.Insert(i.Buffer.Start(), msg)
}
func (i *InfoBuf) YNPrompt(prompt string, donecb func(bool, bool)) {
	if i.HasPrompt {
		i.DonePrompt(true)
	}
	i.Msg = prompt
	i.HasPrompt = true
	i.HasYN = true
	i.HasMessage, i.HasError = false, false
	i.HasGutter = false
	i.YNCallback = donecb
}
func (i *InfoBuf) DonePrompt(canceled bool) {
	hadYN := i.HasYN
	i.HasPrompt = false
	i.HasYN = false
	i.HasGutter = false
	if !hadYN {
		if i.PromptCallback != nil {
			if canceled {
				i.Replace(i.Start(), i.End(), "")
				i.PromptCallback("", true)
				h := i.History[i.PromptType]
				i.History[i.PromptType] = h[:len(h)-1]
			} else {
				resp := string(i.LineBytes(0))
				i.Replace(i.Start(), i.End(), "")
				i.PromptCallback(resp, false)
				h := i.History[i.PromptType]
				h[len(h)-1] = resp
				for j := len(h) - 2; j >= 0; j-- {
					if h[j] == h[len(h)-1] {
						i.History[i.PromptType] = append(h[:j], h[j+1:]...)
						break
					}
				}
			}
		}
	}
	if i.YNCallback != nil && hadYN {
		i.YNCallback(i.YNResp, canceled)
	}
}
func (i *InfoBuf) Reset() {
	i.Msg = ""
	i.HasPrompt, i.HasMessage, i.HasError = false, false, false
	i.HasGutter = false
}