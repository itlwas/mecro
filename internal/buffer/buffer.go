package buffer
import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	luar "layeh.com/gopher-luar"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/zyedidia/micro/v2/internal/config"
	ulua "github.com/zyedidia/micro/v2/internal/lua"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/micro/v2/pkg/highlight"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)
const backupTime = 8000
var (
	OpenBuffers []*Buffer
	LogBuf *Buffer
)
type BufType struct {
	Kind     int
	Readonly bool
	Scratch  bool
	Syntax   bool
}
var (
	BTDefault = BufType{0, false, false, true}
	BTHelp = BufType{1, true, true, true}
	BTLog = BufType{2, true, true, false}
	BTScratch = BufType{3, false, true, false}
	BTRaw = BufType{4, false, true, false}
	BTInfo = BufType{5, false, true, false}
	BTStdout = BufType{6, false, true, true}
	ErrFileTooLarge = errors.New("File is too large to hash")
)
type SharedBuffer struct {
	*LineArray
	ModTime time.Time
	Type BufType
	Path string
	AbsPath string
	name string
	toStdout bool
	Settings map[string]interface{}
	Suggestions   []string
	Completions   []string
	CurSuggestion int
	Messages []*Message
	updateDiffTimer   *time.Timer
	diffBase          []byte
	diffBaseLineCount int
	diffLock          sync.RWMutex
	diff              map[int]DiffStatus
	requestedBackup bool
	ReloadDisabled bool
	isModified bool
	HasSuggestions bool
	Highlighter *highlight.Highlighter
	SyntaxDef *highlight.Def
	ModifiedThisFrame bool
	origHash [md5.Size]byte
}
func (b *SharedBuffer) insert(pos Loc, value []byte) {
	b.isModified = true
	b.HasSuggestions = false
	b.LineArray.insert(pos, value)
	inslines := bytes.Count(value, []byte{'\n'})
	b.MarkModified(pos.Y, pos.Y+inslines)
}
func (b *SharedBuffer) remove(start, end Loc) []byte {
	b.isModified = true
	b.HasSuggestions = false
	defer b.MarkModified(start.Y, end.Y)
	return b.LineArray.remove(start, end)
}
func (b *SharedBuffer) MarkModified(start, end int) {
	b.ModifiedThisFrame = true
	start = util.Clamp(start, 0, len(b.lines)-1)
	end = util.Clamp(end, 0, len(b.lines)-1)
	if b.Settings["syntax"].(bool) && b.SyntaxDef != nil {
		l := -1
		for i := start; i <= end; i++ {
			l = util.Max(b.Highlighter.ReHighlightStates(b, i), l)
		}
		b.Highlighter.HighlightMatches(b, start, l)
	}
	for i := start; i <= end; i++ {
		b.LineArray.invalidateSearchMatches(i)
	}
}
func (b *SharedBuffer) DisableReload() {
	b.ReloadDisabled = true
}
const (
	DSUnchanged    = 0
	DSAdded        = 1
	DSModified     = 2
	DSDeletedAbove = 3
)
type DiffStatus byte
type Buffer struct {
	*EventHandler
	*SharedBuffer
	fini        int32
	cursors     []*Cursor
	curCursor   int
	StartCursor Loc
	OptionCallback func(option string, nativeValue interface{})
	GetVisualX func(loc Loc) int
	LastSearch      string
	LastSearchRegex bool
	HighlightSearch bool
}
func NewBufferFromFileAtLoc(path string, btype BufType, cursorLoc Loc) (*Buffer, error) {
	var err error
	filename := path
	if config.GetGlobalOption("parsecursor").(bool) && cursorLoc.X == -1 && cursorLoc.Y == -1 {
		var cursorPos []string
		filename, cursorPos = util.GetPathAndCursorPosition(filename)
		cursorLoc, err = ParseCursorLocation(cursorPos)
		if err != nil {
			cursorLoc = Loc{-1, -1}
		}
	}
	filename, err = util.ReplaceHome(filename)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filename, os.O_WRONLY, 0)
	readonly := os.IsPermission(err)
	f.Close()
	fileInfo, serr := os.Stat(filename)
	if serr != nil && !os.IsNotExist(serr) {
		return nil, serr
	}
	if serr == nil && fileInfo.IsDir() {
		return nil, errors.New("Error: " + filename + " is a directory and cannot be opened")
	}
	file, err := os.Open(filename)
	if err == nil {
		defer file.Close()
	}
	var buf *Buffer
	if os.IsNotExist(err) {
		buf = NewBufferFromString("", filename, btype)
	} else if err != nil {
		return nil, err
	} else {
		buf = NewBuffer(file, util.FSize(file), filename, cursorLoc, btype)
		if buf == nil {
			return nil, errors.New("could not open file")
		}
	}
	if readonly && prompt != nil {
		prompt.Message(fmt.Sprintf("Warning: file is readonly - %s will be attempted when saving", config.GlobalSettings["sucmd"].(string)))
	}
	return buf, nil
}
func NewBufferFromFile(path string, btype BufType) (*Buffer, error) {
	return NewBufferFromFileAtLoc(path, btype, Loc{-1, -1})
}
func NewBufferFromStringAtLoc(text, path string, btype BufType, cursorLoc Loc) *Buffer {
	return NewBuffer(strings.NewReader(text), int64(len(text)), path, cursorLoc, btype)
}
func NewBufferFromString(text, path string, btype BufType) *Buffer {
	return NewBuffer(strings.NewReader(text), int64(len(text)), path, Loc{-1, -1}, btype)
}
func NewBuffer(r io.Reader, size int64, path string, startcursor Loc, btype BufType) *Buffer {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	b := new(Buffer)
	found := false
	if len(path) > 0 {
		for _, buf := range OpenBuffers {
			if buf.AbsPath == absPath && buf.Type != BTInfo {
				found = true
				b.SharedBuffer = buf.SharedBuffer
				b.EventHandler = buf.EventHandler
			}
		}
	}
	hasBackup := false
	if !found {
		b.SharedBuffer = new(SharedBuffer)
		b.Type = btype
		b.AbsPath = absPath
		b.Path = path
		settings := config.DefaultCommonSettings()
		b.Settings = config.DefaultCommonSettings()
		for k, v := range config.GlobalSettings {
			if _, ok := config.DefaultGlobalOnlySettings[k]; !ok {
				settings[k] = v
				b.Settings[k] = v
			}
		}
		config.InitLocalSettings(settings, absPath)
		b.Settings["readonly"] = settings["readonly"]
		b.Settings["filetype"] = settings["filetype"]
		b.Settings["syntax"] = settings["syntax"]
		enc, err := htmlindex.Get(settings["encoding"].(string))
		if err != nil {
			enc = unicode.UTF8
			b.Settings["encoding"] = "utf-8"
		}
		var ok bool
		hasBackup, ok = b.ApplyBackup(size)
		if !ok {
			return NewBufferFromString("", "", btype)
		}
		if !hasBackup {
			reader := bufio.NewReader(transform.NewReader(r, enc.NewDecoder()))
			var ff FileFormat = FFAuto
			if size == 0 {
				switch settings["fileformat"] {
				case "unix":
					ff = FFUnix
				case "dos":
					ff = FFDos
				}
			}
			b.LineArray = NewLineArray(uint64(size), ff, reader)
		}
		b.EventHandler = NewEventHandler(b.SharedBuffer, b.cursors)
		b.UpdateModTime()
	}
	if b.Settings["readonly"].(bool) && b.Type == BTDefault {
		b.Type.Readonly = true
	}
	switch b.Endings {
	case FFUnix:
		b.Settings["fileformat"] = "unix"
	case FFDos:
		b.Settings["fileformat"] = "dos"
	}
	b.UpdateRules()
	config.InitLocalSettings(b.Settings, b.Path)
	if _, err := os.Stat(filepath.Join(config.ConfigDir, "buffers")); os.IsNotExist(err) {
		os.Mkdir(filepath.Join(config.ConfigDir, "buffers"), os.ModePerm)
	}
	if startcursor.X != -1 && startcursor.Y != -1 {
		b.StartCursor = startcursor
	} else if b.Settings["savecursor"].(bool) || b.Settings["saveundo"].(bool) {
		err := b.Unserialize()
		if err != nil {
			screen.TermMessage(err)
		}
	}
	b.AddCursor(NewCursor(b, b.StartCursor))
	b.GetActiveCursor().Relocate()
	if !b.Settings["fastdirty"].(bool) && !found {
		if size > LargeFileThreshold {
			b.Settings["fastdirty"] = true
		} else if !hasBackup {
			calcHash(b, &b.origHash)
		}
	}
	err = config.RunPluginFn("onBufferOpen", luar.New(ulua.L, b))
	if err != nil {
		screen.TermMessage(err)
	}
	OpenBuffers = append(OpenBuffers, b)
	return b
}
func CloseOpenBuffers() {
	for i, buf := range OpenBuffers {
		buf.Fini()
		OpenBuffers[i] = nil
	}
	OpenBuffers = OpenBuffers[:0]
}
func (b *Buffer) Close() {
	for i, buf := range OpenBuffers {
		if b == buf {
			b.Fini()
			copy(OpenBuffers[i:], OpenBuffers[i+1:])
			OpenBuffers[len(OpenBuffers)-1] = nil
			OpenBuffers = OpenBuffers[:len(OpenBuffers)-1]
			return
		}
	}
}
func (b *Buffer) Fini() {
	if !b.Modified() {
		b.Serialize()
	}
	b.RemoveBackup()
	if b.Type == BTStdout {
		fmt.Fprint(util.Stdout, string(b.Bytes()))
	}
	atomic.StoreInt32(&(b.fini), int32(1))
}
func (b *Buffer) GetName() string {
	name := b.name
	if name == "" {
		if b.Path == "" {
			return "No name"
		}
		name = b.Path
	}
	if b.Settings["basename"].(bool) {
		return path.Base(name)
	}
	return name
}
func (b *Buffer) SetName(s string) {
	b.name = s
}
func (b *Buffer) Insert(start Loc, text string) {
	if !b.Type.Readonly {
		b.EventHandler.cursors = b.cursors
		b.EventHandler.active = b.curCursor
		b.EventHandler.Insert(start, text)
		b.RequestBackup()
	}
}
func (b *Buffer) Remove(start, end Loc) {
	if !b.Type.Readonly {
		b.EventHandler.cursors = b.cursors
		b.EventHandler.active = b.curCursor
		b.EventHandler.Remove(start, end)
		b.RequestBackup()
	}
}
func (b *Buffer) FileType() string {
	return b.Settings["filetype"].(string)
}
func (b *Buffer) ExternallyModified() bool {
	modTime, err := util.GetModTime(b.Path)
	if err == nil {
		return modTime != b.ModTime
	}
	return false
}
func (b *Buffer) UpdateModTime() (err error) {
	b.ModTime, err = util.GetModTime(b.Path)
	return
}
func (b *Buffer) ReOpen() error {
	file, err := os.Open(b.Path)
	if err != nil {
		return err
	}
	enc, err := htmlindex.Get(b.Settings["encoding"].(string))
	if err != nil {
		return err
	}
	reader := bufio.NewReader(transform.NewReader(file, enc.NewDecoder()))
	data, err := ioutil.ReadAll(reader)
	txt := string(data)
	if err != nil {
		return err
	}
	b.EventHandler.ApplyDiff(txt)
	err = b.UpdateModTime()
	if !b.Settings["fastdirty"].(bool) {
		calcHash(b, &b.origHash)
	}
	b.isModified = false
	b.RelocateCursors()
	return err
}
func (b *Buffer) RelocateCursors() {
	for _, c := range b.cursors {
		c.Relocate()
	}
}
func (b *Buffer) DeselectCursors() {
	for _, c := range b.cursors {
		c.Deselect(true)
	}
}
func (b *Buffer) RuneAt(loc Loc) rune {
	line := b.LineBytes(loc.Y)
	if len(line) > 0 {
		i := 0
		for len(line) > 0 {
			r, _, size := util.DecodeCharacter(line)
			line = line[size:]
			if i == loc.X {
				return r
			}
			i++
		}
	}
	return '\n'
}
func (b *Buffer) WordAt(loc Loc) []byte {
	if len(b.LineBytes(loc.Y)) == 0 || !util.IsWordChar(b.RuneAt(loc)) {
		return []byte{}
	}
	start := loc
	end := loc.Move(1, b)
	for start.X > 0 && util.IsWordChar(b.RuneAt(start.Move(-1, b))) {
		start.X--
	}
	lineLen := util.CharacterCount(b.LineBytes(loc.Y))
	for end.X < lineLen && util.IsWordChar(b.RuneAt(end)) {
		end.X++
	}
	return b.Substr(start, end)
}
func (b *Buffer) Modified() bool {
	if b.Type.Scratch {
		return false
	}
	if b.Settings["fastdirty"].(bool) {
		return b.isModified
	}
	var buff [md5.Size]byte
	calcHash(b, &buff)
	return buff != b.origHash
}
func (b *Buffer) Size() int {
	nb := 0
	for i := 0; i < b.LinesNum(); i++ {
		nb += len(b.LineBytes(i))
		if i != b.LinesNum()-1 {
			if b.Endings == FFDos {
				nb++
			}
			nb++
		}
	}
	return nb
}
func calcHash(b *Buffer, out *[md5.Size]byte) error {
	h := md5.New()
	size := 0
	if len(b.lines) > 0 {
		n, e := h.Write(b.lines[0].data)
		if e != nil {
			return e
		}
		size += n
		for _, l := range b.lines[1:] {
			n, e = h.Write([]byte{'\n'})
			if e != nil {
				return e
			}
			size += n
			n, e = h.Write(l.data)
			if e != nil {
				return e
			}
			size += n
		}
	}
	if size > LargeFileThreshold {
		return ErrFileTooLarge
	}
	h.Sum((*out)[:0])
	return nil
}
func parseDefFromFile(f config.RuntimeFile, header *highlight.Header) *highlight.Def {
	data, err := f.Data()
	if err != nil {
		screen.TermMessage("Error loading syntax file " + f.Name() + ": " + err.Error())
		return nil
	}
	if header == nil {
		header, err = highlight.MakeHeaderYaml(data)
		if err != nil {
			screen.TermMessage("Error parsing header for syntax file " + f.Name() + ": " + err.Error())
			return nil
		}
	}
	file, err := highlight.ParseFile(data)
	if err != nil {
		screen.TermMessage("Error parsing syntax file " + f.Name() + ": " + err.Error())
		return nil
	}
	syndef, err := highlight.ParseDef(file, header)
	if err != nil {
		screen.TermMessage("Error parsing syntax file " + f.Name() + ": " + err.Error())
		return nil
	}
	return syndef
}
func findRealRuntimeSyntaxDef(name string, header *highlight.Header) *highlight.Def {
	for _, f := range config.ListRealRuntimeFiles(config.RTSyntax) {
		if f.Name() == name {
			syndef := parseDefFromFile(f, header)
			if syndef != nil {
				return syndef
			}
		}
	}
	return nil
}
func findRuntimeSyntaxDef(name string, header *highlight.Header) *highlight.Def {
	for _, f := range config.ListRuntimeFiles(config.RTSyntax) {
		if f.Name() == name {
			syndef := parseDefFromFile(f, header)
			if syndef != nil {
				return syndef
			}
		}
	}
	return nil
}
func (b *Buffer) UpdateRules() {
	if !b.Type.Syntax {
		return
	}
	ft := b.Settings["filetype"].(string)
	if ft == "off" {
		b.ClearMatches()
		b.SyntaxDef = nil
		return
	}
	type syntaxFileInfo struct {
		header    *highlight.Header
		fileName  string
		syntaxDef *highlight.Def
	}
	fnameMatches := []syntaxFileInfo{}
	headerMatches := []syntaxFileInfo{}
	syntaxFile := ""
	foundDef := false
	var header *highlight.Header
	for _, f := range config.ListRealRuntimeFiles(config.RTSyntax) {
		if f.Name() == "default" {
			continue
		}
		data, err := f.Data()
		if err != nil {
			screen.TermMessage("Error loading syntax file " + f.Name() + ": " + err.Error())
			continue
		}
		header, err = highlight.MakeHeaderYaml(data)
		if err != nil {
			screen.TermMessage("Error parsing header for syntax file " + f.Name() + ": " + err.Error())
			continue
		}
		matchedFileType := false
		matchedFileName := false
		matchedFileHeader := false
		if ft == "unknown" || ft == "" {
			if header.MatchFileName(b.Path) {
				matchedFileName = true
			}
			if len(fnameMatches) == 0 && header.MatchFileHeader(b.lines[0].data) {
				matchedFileHeader = true
			}
		} else if header.FileType == ft {
			matchedFileType = true
		}
		if matchedFileType || matchedFileName || matchedFileHeader {
			file, err := highlight.ParseFile(data)
			if err != nil {
				screen.TermMessage("Error parsing syntax file " + f.Name() + ": " + err.Error())
				continue
			}
			syndef, err := highlight.ParseDef(file, header)
			if err != nil {
				screen.TermMessage("Error parsing syntax file " + f.Name() + ": " + err.Error())
				continue
			}
			if matchedFileType {
				b.SyntaxDef = syndef
				syntaxFile = f.Name()
				foundDef = true
				break
			}
			if matchedFileName {
				fnameMatches = append(fnameMatches, syntaxFileInfo{header, f.Name(), syndef})
			} else if matchedFileHeader {
				headerMatches = append(headerMatches, syntaxFileInfo{header, f.Name(), syndef})
			}
		}
	}
	if !foundDef {
		for _, f := range config.ListRuntimeFiles(config.RTSyntaxHeader) {
			data, err := f.Data()
			if err != nil {
				screen.TermMessage("Error loading syntax header file " + f.Name() + ": " + err.Error())
				continue
			}
			header, err = highlight.MakeHeader(data)
			if err != nil {
				screen.TermMessage("Error reading syntax header file", f.Name(), err)
				continue
			}
			if ft == "unknown" || ft == "" {
				if header.MatchFileName(b.Path) {
					fnameMatches = append(fnameMatches, syntaxFileInfo{header, f.Name(), nil})
				}
				if len(fnameMatches) == 0 && header.MatchFileHeader(b.lines[0].data) {
					headerMatches = append(headerMatches, syntaxFileInfo{header, f.Name(), nil})
				}
			} else if header.FileType == ft {
				syntaxFile = f.Name()
				break
			}
		}
	}
	if syntaxFile == "" {
		matches := fnameMatches
		if len(matches) == 0 {
			matches = headerMatches
		}
		length := len(matches)
		if length > 0 {
			signatureMatch := false
			if length > 1 {
				detectlimit := util.IntOpt(b.Settings["detectlimit"])
				lineCount := len(b.lines)
				limit := lineCount
				if detectlimit > 0 && lineCount > detectlimit {
					limit = detectlimit
				}
			matchLoop:
				for _, m := range matches {
					if m.header.HasFileSignature() {
						for i := 0; i < limit; i++ {
							if m.header.MatchFileSignature(b.lines[i].data) {
								syntaxFile = m.fileName
								if m.syntaxDef != nil {
									b.SyntaxDef = m.syntaxDef
									foundDef = true
								}
								header = m.header
								signatureMatch = true
								break matchLoop
							}
						}
					}
				}
			}
			if length == 1 || !signatureMatch {
				syntaxFile = matches[0].fileName
				if matches[0].syntaxDef != nil {
					b.SyntaxDef = matches[0].syntaxDef
					foundDef = true
				}
				header = matches[0].header
			}
		}
	}
	if syntaxFile != "" && !foundDef {
		b.SyntaxDef = findRuntimeSyntaxDef(syntaxFile, header)
	}
	if b.SyntaxDef != nil && highlight.HasIncludes(b.SyntaxDef) {
		includes := highlight.GetIncludes(b.SyntaxDef)
		var files []*highlight.File
		for _, f := range config.ListRuntimeFiles(config.RTSyntax) {
			data, err := f.Data()
			if err != nil {
				screen.TermMessage("Error loading syntax file " + f.Name() + ": " + err.Error())
				continue
			}
			header, err := highlight.MakeHeaderYaml(data)
			if err != nil {
				screen.TermMessage("Error parsing syntax file " + f.Name() + ": " + err.Error())
				continue
			}
			for _, i := range includes {
				if header.FileType == i {
					file, err := highlight.ParseFile(data)
					if err != nil {
						screen.TermMessage("Error parsing syntax file " + f.Name() + ": " + err.Error())
						continue
					}
					files = append(files, file)
					break
				}
			}
			if len(files) >= len(includes) {
				break
			}
		}
		highlight.ResolveIncludes(b.SyntaxDef, files)
	}
	if b.Highlighter == nil || syntaxFile != "" {
		if b.SyntaxDef != nil {
			b.Settings["filetype"] = b.SyntaxDef.FileType
		} else {
			b.SyntaxDef = findRealRuntimeSyntaxDef("default", nil)
			if b.SyntaxDef == nil {
				b.SyntaxDef = findRuntimeSyntaxDef("default", nil)
			}
		}
	}
	if b.SyntaxDef != nil {
		b.Highlighter = highlight.NewHighlighter(b.SyntaxDef)
		if b.Settings["syntax"].(bool) {
			go func() {
				b.Highlighter.HighlightStates(b)
				b.Highlighter.HighlightMatches(b, 0, b.End().Y)
				screen.Redraw()
			}()
		}
	}
}
func (b *Buffer) ClearMatches() {
	for i := range b.lines {
		b.SetMatch(i, nil)
		b.SetState(i, nil)
	}
}
func (b *Buffer) IndentString(tabsize int) string {
	if b.Settings["tabstospaces"].(bool) {
		return util.Spaces(tabsize)
	}
	return "\t"
}
func (b *Buffer) SetCursors(c []*Cursor) {
	b.cursors = c
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
}
func (b *Buffer) AddCursor(c *Cursor) {
	b.cursors = append(b.cursors, c)
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
	b.UpdateCursors()
}
func (b *Buffer) SetCurCursor(n int) {
	b.curCursor = n
}
func (b *Buffer) GetActiveCursor() *Cursor {
	return b.cursors[b.curCursor]
}
func (b *Buffer) GetCursor(n int) *Cursor {
	return b.cursors[n]
}
func (b *Buffer) GetCursors() []*Cursor {
	return b.cursors
}
func (b *Buffer) NumCursors() int {
	return len(b.cursors)
}
func (b *Buffer) MergeCursors() {
	var cursors []*Cursor
	for i := 0; i < len(b.cursors); i++ {
		c1 := b.cursors[i]
		if c1 != nil {
			for j := 0; j < len(b.cursors); j++ {
				c2 := b.cursors[j]
				if c2 != nil && i != j && c1.Loc == c2.Loc {
					b.cursors[j] = nil
				}
			}
			cursors = append(cursors, c1)
		}
	}
	b.cursors = cursors
	for i := range b.cursors {
		b.cursors[i].Num = i
	}
	if b.curCursor >= len(b.cursors) {
		b.curCursor = len(b.cursors) - 1
	}
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
}
func (b *Buffer) UpdateCursors() {
	b.EventHandler.cursors = b.cursors
	b.EventHandler.active = b.curCursor
	for i, c := range b.cursors {
		c.Num = i
	}
}
func (b *Buffer) RemoveCursor(i int) {
	copy(b.cursors[i:], b.cursors[i+1:])
	b.cursors[len(b.cursors)-1] = nil
	b.cursors = b.cursors[:len(b.cursors)-1]
	b.curCursor = util.Clamp(b.curCursor, 0, len(b.cursors)-1)
	b.UpdateCursors()
}
func (b *Buffer) ClearCursors() {
	for i := 1; i < len(b.cursors); i++ {
		b.cursors[i] = nil
	}
	b.cursors = b.cursors[:1]
	b.UpdateCursors()
	b.curCursor = 0
	b.GetActiveCursor().ResetSelection()
}
func (b *Buffer) MoveLinesUp(start int, end int) {
	if start < 1 || start >= end || end > len(b.lines) {
		return
	}
	l := string(b.LineBytes(start - 1))
	if end == len(b.lines) {
		b.insert(
			Loc{
				util.CharacterCount(b.lines[end-1].data),
				end - 1,
			},
			[]byte{'\n'},
		)
	}
	b.Insert(
		Loc{0, end},
		l+"\n",
	)
	b.Remove(
		Loc{0, start - 1},
		Loc{0, start},
	)
}
func (b *Buffer) MoveLinesDown(start int, end int) {
	if start < 0 || start >= end || end >= len(b.lines) {
		return
	}
	l := string(b.LineBytes(end))
	b.Insert(
		Loc{0, start},
		l+"\n",
	)
	end++
	b.Remove(
		Loc{0, end},
		Loc{0, end + 1},
	)
}
var BracePairs = [][2]rune{
	{'(', ')'},
	{'{', '}'},
	{'[', ']'},
}
func (b *Buffer) FindMatchingBrace(braceType [2]rune, start Loc) (Loc, bool, bool) {
	curLine := []rune(string(b.LineBytes(start.Y)))
	startChar := ' '
	if start.X >= 0 && start.X < len(curLine) {
		startChar = curLine[start.X]
	}
	leftChar := ' '
	if start.X-1 >= 0 && start.X-1 < len(curLine) {
		leftChar = curLine[start.X-1]
	}
	var i int
	if startChar == braceType[0] || (leftChar == braceType[0] && startChar != braceType[1]) {
		for y := start.Y; y < b.LinesNum(); y++ {
			l := []rune(string(b.LineBytes(y)))
			xInit := 0
			if y == start.Y {
				if startChar == braceType[0] {
					xInit = start.X
				} else {
					xInit = start.X - 1
				}
			}
			for x := xInit; x < len(l); x++ {
				r := l[x]
				if r == braceType[0] {
					i++
				} else if r == braceType[1] {
					i--
					if i == 0 {
						if startChar == braceType[0] {
							return Loc{x, y}, false, true
						}
						return Loc{x, y}, true, true
					}
				}
			}
		}
	} else if startChar == braceType[1] || leftChar == braceType[1] {
		for y := start.Y; y >= 0; y-- {
			l := []rune(string(b.lines[y].data))
			xInit := len(l) - 1
			if y == start.Y {
				if startChar == braceType[1] {
					xInit = start.X
				} else {
					xInit = start.X - 1
				}
			}
			for x := xInit; x >= 0; x-- {
				r := l[x]
				if r == braceType[1] {
					i++
				} else if r == braceType[0] {
					i--
					if i == 0 {
						if startChar == braceType[1] {
							return Loc{x, y}, false, true
						}
						return Loc{x, y}, true, true
					}
				}
			}
		}
	}
	return start, true, false
}
func (b *Buffer) Retab() {
	toSpaces := b.Settings["tabstospaces"].(bool)
	tabsize := util.IntOpt(b.Settings["tabsize"])
	dirty := false
	for i := 0; i < b.LinesNum(); i++ {
		l := b.LineBytes(i)
		ws := util.GetLeadingWhitespace(l)
		if len(ws) != 0 {
			if toSpaces {
				ws = bytes.ReplaceAll(ws, []byte{'\t'}, bytes.Repeat([]byte{' '}, tabsize))
			} else {
				ws = bytes.ReplaceAll(ws, bytes.Repeat([]byte{' '}, tabsize), []byte{'\t'})
			}
		}
		l = bytes.TrimLeft(l, " \t")
		b.Lock()
		b.lines[i].data = append(ws, l...)
		b.Unlock()
		b.MarkModified(i, i)
		dirty = true
	}
	b.isModified = dirty
}
func ParseCursorLocation(cursorPositions []string) (Loc, error) {
	startpos := Loc{0, 0}
	var err error
	if cursorPositions == nil {
		return startpos, errors.New("No cursor positions were provided.")
	}
	startpos.Y, err = strconv.Atoi(cursorPositions[0])
	startpos.Y--
	if err == nil {
		if len(cursorPositions) > 1 {
			startpos.X, err = strconv.Atoi(cursorPositions[1])
			if startpos.X > 0 {
				startpos.X--
			}
		}
	}
	return startpos, err
}
func (b *Buffer) Line(i int) string {
	return string(b.LineBytes(i))
}
func (b *Buffer) Write(bytes []byte) (n int, err error) {
	b.EventHandler.InsertBytes(b.End(), bytes)
	return len(bytes), nil
}
func (b *Buffer) updateDiffSync() {
	b.diffLock.Lock()
	defer b.diffLock.Unlock()
	b.diff = make(map[int]DiffStatus)
	if b.diffBase == nil {
		return
	}
	differ := dmp.New()
	baseRunes, bufferRunes, _ := differ.DiffLinesToRunes(string(b.diffBase), string(b.Bytes()))
	diffs := differ.DiffMainRunes(baseRunes, bufferRunes, false)
	lineN := 0
	for _, diff := range diffs {
		lineCount := len([]rune(diff.Text))
		switch diff.Type {
		case dmp.DiffEqual:
			lineN += lineCount
		case dmp.DiffInsert:
			var status DiffStatus
			if b.diff[lineN] == DSDeletedAbove {
				status = DSModified
			} else {
				status = DSAdded
			}
			for i := 0; i < lineCount; i++ {
				b.diff[lineN] = status
				lineN++
			}
		case dmp.DiffDelete:
			b.diff[lineN] = DSDeletedAbove
		}
	}
}
func (b *Buffer) UpdateDiff(callback func(bool)) {
	if b.updateDiffTimer != nil {
		return
	}
	lineCount := b.LinesNum()
	if b.diffBaseLineCount > lineCount {
		lineCount = b.diffBaseLineCount
	}
	if lineCount < 1000 {
		b.updateDiffSync()
		callback(true)
	} else if lineCount < 30000 {
		b.updateDiffTimer = time.AfterFunc(500*time.Millisecond, func() {
			b.updateDiffTimer = nil
			b.updateDiffSync()
			callback(false)
		})
	} else {
		b.diffLock.Lock()
		b.diff = make(map[int]DiffStatus)
		b.diffLock.Unlock()
		callback(true)
	}
}
func (b *Buffer) SetDiffBase(diffBase []byte) {
	b.diffBase = diffBase
	if diffBase == nil {
		b.diffBaseLineCount = 0
	} else {
		b.diffBaseLineCount = strings.Count(string(diffBase), "\n")
	}
	b.UpdateDiff(func(synchronous bool) {
		screen.Redraw()
	})
}
func (b *Buffer) DiffStatus(lineN int) DiffStatus {
	b.diffLock.RLock()
	defer b.diffLock.RUnlock()
	return b.diff[lineN]
}
func (b *Buffer) FindNextDiffLine(startLine int, forward bool) (int, error) {
	if b.diff == nil {
		return 0, errors.New("no diff data")
	}
	startStatus, ok := b.diff[startLine]
	if !ok {
		startStatus = DSUnchanged
	}
	curLine := startLine
	for {
		curStatus, ok := b.diff[curLine]
		if !ok {
			curStatus = DSUnchanged
		}
		if curLine < 0 || curLine > b.LinesNum() {
			return 0, errors.New("no next diff hunk")
		}
		if curStatus != startStatus {
			if startStatus != DSUnchanged && curStatus == DSUnchanged {
				startStatus = DSUnchanged
			} else {
				return curLine, nil
			}
		}
		if forward {
			curLine++
		} else {
			curLine--
		}
	}
}
func (b *Buffer) SearchMatch(pos Loc) bool {
	return b.LineArray.SearchMatch(b, pos)
}
func WriteLog(s string) {
	LogBuf.EventHandler.Insert(LogBuf.End(), s)
}
func GetLogBuf() *Buffer {
	return LogBuf
}