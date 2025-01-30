package buffer
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/util"
	"golang.org/x/text/encoding"
)
const backupMsg = `A backup was detected for %s on %s.
This likely means that mecro crashed while editing this file,
or another instance of mecro is currently editing this file.
Options: [r]ecover, [i]gnore, [a]bort: `
var backupRequestChan chan *Buffer
func backupThread() {
	for {
		time.Sleep(time.Second * 8)
		for len(backupRequestChan) > 0 {
			b := <-backupRequestChan
			bfini := atomic.LoadInt32(&(b.fini)) != 0
			if !bfini {
				b.Backup()
			}
		}
	}
}
func init() {
	backupRequestChan = make(chan *Buffer, 10)
	go backupThread()
}
func (b *Buffer) RequestBackup() {
	if !b.requestedBackup {
		select {
		case backupRequestChan <- b:
		default:
		}
		b.requestedBackup = true
	}
}
func (b *Buffer) Backup() error {
	if !b.Settings["backup"].(bool) || b.Path == "" || b.Type != BTDefault {
		return nil
	}
	backupdir, err := util.ReplaceHome(b.Settings["backupdir"].(string))
	if backupdir == "" || err != nil {
		backupdir = filepath.Join(config.ConfigDir, "backups")
	}
	if _, err := os.Stat(backupdir); os.IsNotExist(err) {
		os.Mkdir(backupdir, os.ModePerm)
	}
	name := filepath.Join(backupdir, util.EscapePath(b.AbsPath))
	err = overwriteFile(name, encoding.Nop, func(file io.Writer) (e error) {
		if len(b.lines) == 0 {
			return
		}
		eol := []byte{'\n'}
		if _, e = file.Write(b.lines[0].data); e != nil {
			return
		}
		for _, l := range b.lines[1:] {
			if _, e = file.Write(eol); e != nil {
				return
			}
			if _, e = file.Write(l.data); e != nil {
				return
			}
		}
		return
	}, false)
	b.requestedBackup = false
	return err
}
func (b *Buffer) RemoveBackup() {
	if !b.Settings["backup"].(bool) || b.Settings["permbackup"].(bool) || b.Path == "" || b.Type != BTDefault {
		return
	}
	f := filepath.Join(config.ConfigDir, "backups", util.EscapePath(b.AbsPath))
	os.Remove(f)
}
func (b *Buffer) ApplyBackup(fsize int64) (bool, bool) {
	if b.Settings["backup"].(bool) && !b.Settings["permbackup"].(bool) && len(b.Path) > 0 && b.Type == BTDefault {
		backupfile := filepath.Join(config.ConfigDir, "backups", util.EscapePath(b.AbsPath))
		if info, err := os.Stat(backupfile); err == nil {
			backup, err := os.Open(backupfile)
			if err == nil {
				defer backup.Close()
				t := info.ModTime()
				msg := fmt.Sprintf(backupMsg, t.Format("Mon Jan _2 at 15:04, 2006"), util.EscapePath(b.AbsPath))
				choice := screen.TermPrompt(msg, []string{"r", "i", "a", "recover", "ignore", "abort"}, true)
				if choice%3 == 0 {
					b.LineArray = NewLineArray(uint64(fsize), FFAuto, backup)
					b.isModified = true
					return true, true
				} else if choice%3 == 1 {
					os.Remove(backupfile)
				} else if choice%3 == 2 {
					return false, false
				}
			}
		}
	}
	return false, true
}