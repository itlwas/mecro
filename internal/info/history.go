package info
import (
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/util"
)
func (i *InfoBuf) LoadHistory() {
	if config.GetGlobalOption("savehistory").(bool) {
		file, err := os.Open(filepath.Join(config.ConfigDir, "buffers", "history"))
		var decodedMap map[string][]string
		if err == nil {
			defer file.Close()
			decoder := gob.NewDecoder(file)
			err = decoder.Decode(&decodedMap)
			if err != nil {
				i.Error("Error loading history:", err)
				return
			}
		}
		if decodedMap != nil {
			i.History = decodedMap
		} else {
			i.History = make(map[string][]string)
		}
	} else {
		i.History = make(map[string][]string)
	}
}
func (i *InfoBuf) SaveHistory() {
	if config.GetGlobalOption("savehistory").(bool) {
		for k, v := range i.History {
			if len(v) > 100 {
				i.History[k] = v[len(i.History[k])-100:]
			}
		}
		file, err := os.Create(filepath.Join(config.ConfigDir, "buffers", "history"))
		if err == nil {
			defer file.Close()
			encoder := gob.NewEncoder(file)
			err = encoder.Encode(i.History)
			if err != nil {
				i.Error("Error saving history:", err)
				return
			}
		}
	}
}
func (i *InfoBuf) AddToHistory(ptype string, item string) {
	if i.HasPrompt && i.PromptType == ptype {
		return
	}
	if _, ok := i.History[ptype]; !ok {
		i.History[ptype] = []string{item}
	} else {
		i.History[ptype] = append(i.History[ptype], item)
		h := i.History[ptype]
		for j := len(h) - 2; j >= 0; j-- {
			if h[j] == h[len(h)-1] {
				i.History[ptype] = append(h[:j], h[j+1:]...)
				break
			}
		}
	}
}
func (i *InfoBuf) UpHistory(history []string) {
	if i.HistoryNum > 0 && i.HasPrompt && !i.HasYN {
		i.HistoryNum--
		i.Replace(i.Start(), i.End(), history[i.HistoryNum])
		i.Buffer.GetActiveCursor().GotoLoc(i.End())
	}
}
func (i *InfoBuf) DownHistory(history []string) {
	if i.HistoryNum < len(history)-1 && i.HasPrompt && !i.HasYN {
		i.HistoryNum++
		i.Replace(i.Start(), i.End(), history[i.HistoryNum])
		i.Buffer.GetActiveCursor().GotoLoc(i.End())
	}
}
func (i *InfoBuf) SearchUpHistory(history []string) {
	if i.HistoryNum > 0 && i.HasPrompt && !i.HasYN {
		i.searchHistory(history, false)
	}
}
func (i *InfoBuf) SearchDownHistory(history []string) {
	if i.HistoryNum < len(history)-1 && i.HasPrompt && !i.HasYN {
		i.searchHistory(history, true)
	}
}
func (i *InfoBuf) searchHistory(history []string, down bool) {
	line := string(i.LineBytes(0))
	c := i.Buffer.GetActiveCursor()
	if !i.HistorySearch || !strings.HasPrefix(line, i.HistorySearchPrefix) {
		i.HistorySearch = true
		i.HistorySearchPrefix = util.SliceStartStr(line, c.X)
	}
	found := -1
	if down {
		for j := i.HistoryNum + 1; j < len(history); j++ {
			if strings.HasPrefix(history[j], i.HistorySearchPrefix) {
				found = j
				break
			}
		}
	} else {
		for j := i.HistoryNum - 1; j >= 0; j-- {
			if strings.HasPrefix(history[j], i.HistorySearchPrefix) {
				found = j
				break
			}
		}
	}
	if found != -1 {
		i.HistoryNum = found
		i.Replace(i.Start(), i.End(), history[found])
		c.GotoLoc(i.End())
	}
}