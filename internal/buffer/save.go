package buffer
import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"unicode"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/util"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)
const LargeFileThreshold = 50000
func overwriteFile(name string, enc encoding.Encoding, fn func(io.Writer) error, withSudo bool) (err error) {
	var writeCloser io.WriteCloser
	var screenb bool
	var cmd *exec.Cmd
	if withSudo {
		cmd = exec.Command(config.GlobalSettings["sucmd"].(string), "dd", "bs=4k", "of="+name)
		if writeCloser, err = cmd.StdinPipe(); err != nil {
			return
		}
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			<-c
			cmd.Process.Kill()
		}()
		screenb = screen.TempFini()
		if e := cmd.Start(); e != nil && err == nil {
			screen.TempStart(screenb)
			return err
		}
	} else if writeCloser, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666); err != nil {
		return
	}
	w := bufio.NewWriter(transform.NewWriter(writeCloser, enc.NewEncoder()))
	err = fn(w)
	if err2 := w.Flush(); err2 != nil && err == nil {
		err = err2
	}
	if !withSudo {
		f := writeCloser.(*os.File)
		if err2 := f.Sync(); err2 != nil && err == nil {
			err = err2
		}
	}
	if err2 := writeCloser.Close(); err2 != nil && err == nil {
		err = err2
	}
	if withSudo {
		err := cmd.Wait()
		screen.TempStart(screenb)
		if err != nil {
			return err
		}
	}
	return
}
func (b *Buffer) Save() error {
	return b.SaveAs(b.Path)
}
func (b *Buffer) AutoSave() error {
	return b.saveToFile(b.Path, false, true)
}
func (b *Buffer) SaveAs(filename string) error {
	return b.saveToFile(filename, false, false)
}
func (b *Buffer) SaveWithSudo() error {
	return b.SaveAsWithSudo(b.Path)
}
func (b *Buffer) SaveAsWithSudo(filename string) error {
	return b.saveToFile(filename, true, false)
}
func (b *Buffer) saveToFile(filename string, withSudo bool, autoSave bool) error {
	var err error
	if b.Type.Readonly {
		return errors.New("Cannot save readonly buffer")
	}
	if b.Type.Scratch {
		return errors.New("Cannot save scratch buffer")
	}
	if withSudo && runtime.GOOS == "windows" {
		return errors.New("Save with sudo not supported on Windows")
	}
	if !autoSave && b.Settings["rmtrailingws"].(bool) {
		for i, l := range b.lines {
			leftover := util.CharacterCount(bytes.TrimRightFunc(l.data, unicode.IsSpace))
			linelen := util.CharacterCount(l.data)
			b.Remove(Loc{leftover, i}, Loc{linelen, i})
		}
		b.RelocateCursors()
	}
	if b.Settings["eofnewline"].(bool) {
		end := b.End()
		if b.RuneAt(Loc{end.X - 1, end.Y}) != '\n' {
			b.insert(end, []byte{'\n'})
		}
	}
	defer func() {
		b.ModTime, _ = util.GetModTime(filename)
		err = b.Serialize()
	}()
	absFilename, _ := util.ReplaceHome(filename)
	if dirname := filepath.Dir(absFilename); dirname != "." {
		if _, statErr := os.Stat(dirname); os.IsNotExist(statErr) {
			if b.Settings["mkparents"].(bool) {
				if mkdirallErr := os.MkdirAll(dirname, os.ModePerm); mkdirallErr != nil {
					return mkdirallErr
				}
			} else {
				return errors.New("Parent dirs don't exist, enable 'mkparents' for auto creation")
			}
		}
	}
	var fileSize int
	enc, err := htmlindex.Get(b.Settings["encoding"].(string))
	if err != nil {
		return err
	}
	fwriter := func(file io.Writer) (e error) {
		if len(b.lines) == 0 {
			return
		}
		var eol []byte
		if b.Endings == FFDos {
			eol = []byte{'\r', '\n'}
		} else {
			eol = []byte{'\n'}
		}
		if fileSize, e = file.Write(b.lines[0].data); e != nil {
			return
		}
		for _, l := range b.lines[1:] {
			if _, e = file.Write(eol); e != nil {
				return
			}
			if _, e = file.Write(l.data); e != nil {
				return
			}
			fileSize += len(eol) + len(l.data)
		}
		return
	}
	if err = overwriteFile(absFilename, enc, fwriter, withSudo); err != nil {
		return err
	}
	if !b.Settings["fastdirty"].(bool) {
		if fileSize > LargeFileThreshold {
			b.Settings["fastdirty"] = true
		} else {
			calcHash(b, &b.origHash)
		}
	}
	b.Path = filename
	absPath, _ := filepath.Abs(filename)
	b.AbsPath = absPath
	b.isModified = false
	b.UpdateRules()
	return err
}