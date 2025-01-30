package util
import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"github.com/blang/semver"
	runewidth "github.com/mattn/go-runewidth"
)
var (
	Version = "0.0.0-unknown"
	SemVersion semver.Version
	CompileDate = "Unknown"
	Debug = "OFF"
	FakeCursor = false
	Stdout *bytes.Buffer
)
func init() {
	var err error
	SemVersion, err = semver.Make(Version)
	if err != nil {
		fmt.Println("Invalid version: ", Version, err)
	}
	_, wt := os.LookupEnv("WT_SESSION")
	if runtime.GOOS == "windows" && !wt {
		FakeCursor = true
	}
	Stdout = new(bytes.Buffer)
}
func SliceEnd(slc []byte, index int) []byte {
	len := len(slc)
	i := 0
	totalSize := 0
	for totalSize < len {
		if i >= index {
			return slc[totalSize:]
		}
		_, _, size := DecodeCharacter(slc[totalSize:])
		totalSize += size
		i++
	}
	return slc[totalSize:]
}
func SliceEndStr(str string, index int) string {
	len := len(str)
	i := 0
	totalSize := 0
	for totalSize < len {
		if i >= index {
			return str[totalSize:]
		}
		_, _, size := DecodeCharacterInString(str[totalSize:])
		totalSize += size
		i++
	}
	return str[totalSize:]
}
func SliceStart(slc []byte, index int) []byte {
	len := len(slc)
	i := 0
	totalSize := 0
	for totalSize < len {
		if i >= index {
			return slc[:totalSize]
		}
		_, _, size := DecodeCharacter(slc[totalSize:])
		totalSize += size
		i++
	}
	return slc[:totalSize]
}
func SliceStartStr(str string, index int) string {
	len := len(str)
	i := 0
	totalSize := 0
	for totalSize < len {
		if i >= index {
			return str[:totalSize]
		}
		_, _, size := DecodeCharacterInString(str[totalSize:])
		totalSize += size
		i++
	}
	return str[:totalSize]
}
func SliceVisualEnd(b []byte, n, tabsize int) ([]byte, int, int) {
	width := 0
	i := 0
	for len(b) > 0 {
		r, _, size := DecodeCharacter(b)
		w := 0
		switch r {
		case '\t':
			ts := tabsize - (width % tabsize)
			w = ts
		default:
			w = runewidth.RuneWidth(r)
		}
		if width+w > n {
			return b, n - width, i
		}
		width += w
		b = b[size:]
		i++
	}
	return b, n - width, i
}
func Abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func StringWidth(b []byte, n, tabsize int) int {
	if n <= 0 {
		return 0
	}
	i := 0
	width := 0
	for len(b) > 0 {
		r, _, size := DecodeCharacter(b)
		b = b[size:]
		switch r {
		case '\t':
			ts := tabsize - (width % tabsize)
			width += ts
		default:
			width += runewidth.RuneWidth(r)
		}
		i++
		if i == n {
			return width
		}
	}
	return width
}
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func FSize(f *os.File) int64 {
	fi, _ := f.Stat()
	return fi.Size()
}
func IsWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
}
func Spaces(n int) string {
	return strings.Repeat(" ", n)
}
func IsSpaces(str []byte) bool {
	for _, c := range str {
		if c != ' ' {
			return false
		}
	}
	return true
}
func IsSpacesOrTabs(str []byte) bool {
	for _, c := range str {
		if c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}
func IsWhitespace(c rune) bool {
	return unicode.IsSpace(c)
}
func IsBytesWhitespace(b []byte) bool {
	for _, c := range b {
		if !IsWhitespace(rune(c)) {
			return false
		}
	}
	return true
}
func RunePos(b []byte, i int) int {
	return CharacterCount(b[:i])
}
func MakeRelative(path, base string) (string, error) {
	if len(path) > 0 {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return path, err
		}
		return rel, nil
	}
	return path, nil
}
func ReplaceHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	var userData *user.User
	var err error
	homeString := strings.Split(path, "/")[0]
	if homeString == "~" {
		userData, err = user.Current()
		if err != nil {
			return "", errors.New("Could not find user: " + err.Error())
		}
	} else {
		userData, err = user.Lookup(homeString[1:])
		if err != nil {
			return "", errors.New("Could not find user: " + err.Error())
		}
	}
	home := userData.HomeDir
	return strings.Replace(path, homeString, home, 1), nil
}
func GetPathAndCursorPosition(path string) (string, []string) {
	re := regexp.MustCompile(`([\s\S]+?)(?::(\d+))(?::(\d+))?$`)
	match := re.FindStringSubmatch(path)
	if len(match) == 0 {
		return path, nil
	} else if match[len(match)-1] != "" {
		return match[1], match[2:]
	}
	return match[1], []string{match[2], "0"}
}
func GetModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now(), err
	}
	return info.ModTime(), nil
}
func EscapePath(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, ":", "%")
	}
	return strings.ReplaceAll(path, "/", "%")
}
func GetLeadingWhitespace(b []byte) []byte {
	ws := []byte{}
	for len(b) > 0 {
		r, _, size := DecodeCharacter(b)
		if r == ' ' || r == '\t' {
			ws = append(ws, byte(r))
		} else {
			break
		}
		b = b[size:]
	}
	return ws
}
func GetTrailingWhitespace(b []byte) []byte {
	ws := []byte{}
	for len(b) > 0 {
		r, size := utf8.DecodeLastRune(b)
		if IsWhitespace(r) {
			ws = append([]byte(string(r)), ws...)
		} else {
			break
		}
		b = b[:len(b)-size]
	}
	return ws
}
func HasTrailingWhitespace(b []byte) bool {
	r, _ := utf8.DecodeLastRune(b)
	return IsWhitespace(r)
}
func IntOpt(opt interface{}) int {
	return int(opt.(float64))
}
func GetCharPosInLine(b []byte, visualPos int, tabsize int) int {
	i := 0
	width := 0
	for len(b) > 0 {
		r, _, size := DecodeCharacter(b)
		b = b[size:]
		switch r {
		case '\t':
			ts := tabsize - (width % tabsize)
			width += ts
		default:
			width += runewidth.RuneWidth(r)
		}
		if width >= visualPos {
			if width == visualPos {
				i++
			}
			break
		}
		i++
	}
	return i
}
func ParseBool(str string) (bool, error) {
	if str == "on" {
		return true, nil
	}
	if str == "off" {
		return false, nil
	}
	return strconv.ParseBool(str)
}
func Clamp(val, min, max int) int {
	if val < min {
		val = min
	} else if val > max {
		val = max
	}
	return val
}
func IsNonAlphaNumeric(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsNumber(c) && c != '_'
}
func IsAutocomplete(c rune) bool {
	return c == '.' || !IsNonAlphaNumeric(c)
}
func ParseSpecial(s string) string {
	return strings.ReplaceAll(s, "\\t", "\t")
}
func String(s []byte) string {
	return string(s)
}
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	os.MkdirAll(dest, 0755)
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		path := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}
	return nil
}
func HttpRequest(method string, url string, headers []string) (resp *http.Response, err error) {
	client := http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}
	return client.Do(req)
}