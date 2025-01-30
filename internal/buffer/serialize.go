package buffer
import (
	"encoding/gob"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
	"golang.org/x/text/encoding"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/util"
)
type SerializedBuffer struct {
	EventHandler *EventHandler
	Cursor       Loc
	ModTime      time.Time
}
func (b *Buffer) Serialize() error {
	if !b.Settings["savecursor"].(bool) && !b.Settings["saveundo"].(bool) {
		return nil
	}
	if b.Path == "" {
		return nil
	}
	name := filepath.Join(config.ConfigDir, "buffers", util.EscapePath(b.AbsPath))
	return overwriteFile(name, encoding.Nop, func(file io.Writer) error {
		err := gob.NewEncoder(file).Encode(SerializedBuffer{
			b.EventHandler,
			b.GetActiveCursor().Loc,
			b.ModTime,
		})
		return err
	}, false)
}
func (b *Buffer) Unserialize() error {
	if b.Path == "" {
		return nil
	}
	file, err := os.Open(filepath.Join(config.ConfigDir, "buffers", util.EscapePath(b.AbsPath)))
	if err == nil {
		defer file.Close()
		var buffer SerializedBuffer
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(&buffer)
		if err != nil {
			return errors.New(err.Error() + "\nYou may want to remove the files in ~/.config/mecro/buffers if\nthis problem persists.")
		}
		if b.Settings["savecursor"].(bool) {
			b.StartCursor = buffer.Cursor
		}
		if b.Settings["saveundo"].(bool) {
			if b.ModTime == buffer.ModTime {
				b.EventHandler = buffer.EventHandler
				b.EventHandler.cursors = b.cursors
				b.EventHandler.buf = b.SharedBuffer
			}
		}
	}
	return nil
}