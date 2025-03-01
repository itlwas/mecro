package config
import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	rt "github.com/zyedidia/micro/v2/runtime"
)
const (
	RTColorscheme  = 0
	RTSyntax       = 1
	RTHelp         = 2
	RTPlugin       = 3
	RTSyntaxHeader = 4
)
var (
	NumTypes = 5
)
type RTFiletype int
type RuntimeFile interface {
	Name() string
	Data() ([]byte, error)
}
var allFiles [][]RuntimeFile
var realFiles [][]RuntimeFile
func init() {
	initRuntimeVars()
}
func initRuntimeVars() {
	allFiles = make([][]RuntimeFile, NumTypes)
	realFiles = make([][]RuntimeFile, NumTypes)
}
func NewRTFiletype() int {
	NumTypes++
	allFiles = append(allFiles, []RuntimeFile{})
	realFiles = append(realFiles, []RuntimeFile{})
	return NumTypes - 1
}
type realFile string
type assetFile string
type namedFile struct {
	realFile
	name string
}
type memoryFile struct {
	name string
	data []byte
}
func (mf memoryFile) Name() string {
	return mf.name
}
func (mf memoryFile) Data() ([]byte, error) {
	return mf.data, nil
}
func (rf realFile) Name() string {
	fn := filepath.Base(string(rf))
	return fn[:len(fn)-len(filepath.Ext(fn))]
}
func (rf realFile) Data() ([]byte, error) {
	return ioutil.ReadFile(string(rf))
}
func (af assetFile) Name() string {
	fn := path.Base(string(af))
	return fn[:len(fn)-len(path.Ext(fn))]
}
func (af assetFile) Data() ([]byte, error) {
	return rt.Asset(string(af))
}
func (nf namedFile) Name() string {
	return nf.name
}
func AddRuntimeFile(fileType RTFiletype, file RuntimeFile) {
	allFiles[fileType] = append(allFiles[fileType], file)
}
func AddRealRuntimeFile(fileType RTFiletype, file RuntimeFile) {
	allFiles[fileType] = append(allFiles[fileType], file)
	realFiles[fileType] = append(realFiles[fileType], file)
}
func AddRuntimeFilesFromDirectory(fileType RTFiletype, directory, pattern string) {
	files, _ := ioutil.ReadDir(directory)
	for _, f := range files {
		if ok, _ := filepath.Match(pattern, f.Name()); !f.IsDir() && ok {
			fullPath := filepath.Join(directory, f.Name())
			AddRealRuntimeFile(fileType, realFile(fullPath))
		}
	}
}
func AddRuntimeFilesFromAssets(fileType RTFiletype, directory, pattern string) {
	files, err := rt.AssetDir(directory)
	if err != nil {
		return
	}
assetLoop:
	for _, f := range files {
		if ok, _ := path.Match(pattern, f); ok {
			af := assetFile(path.Join(directory, f))
			for _, rf := range realFiles[fileType] {
				if af.Name() == rf.Name() {
					continue assetLoop
				}
			}
			AddRuntimeFile(fileType, af)
		}
	}
}
func FindRuntimeFile(fileType RTFiletype, name string) RuntimeFile {
	for _, f := range ListRuntimeFiles(fileType) {
		if f.Name() == name {
			return f
		}
	}
	return nil
}
func ListRuntimeFiles(fileType RTFiletype) []RuntimeFile {
	return allFiles[fileType]
}
func ListRealRuntimeFiles(fileType RTFiletype) []RuntimeFile {
	return realFiles[fileType]
}
func InitRuntimeFiles(user bool) {
	add := func(fileType RTFiletype, dir, pattern string) {
		if user {
			AddRuntimeFilesFromDirectory(fileType, filepath.Join(ConfigDir, dir), pattern)
		}
		AddRuntimeFilesFromAssets(fileType, path.Join("runtime", dir), pattern)
	}
	initRuntimeVars()
	add(RTColorscheme, "colorschemes", "*.micro")
	add(RTSyntax, "syntax", "*.yaml")
	add(RTSyntaxHeader, "syntax", "*.hdr")
	add(RTHelp, "help", "*.md")
}
func InitPlugins() {
	Plugins = Plugins[:0]
	initlua := filepath.Join(ConfigDir, "init.lua")
	if _, err := os.Stat(initlua); !os.IsNotExist(err) {
		p := new(Plugin)
		p.Name = "initlua"
		p.DirName = "initlua"
		p.Srcs = append(p.Srcs, realFile(initlua))
		Plugins = append(Plugins, p)
	}
	plugdir := filepath.Join(ConfigDir, "plug")
	files, _ := ioutil.ReadDir(plugdir)
	isID := regexp.MustCompile(`^[_A-Za-z0-9]+$`).MatchString
	for _, d := range files {
		plugpath := filepath.Join(plugdir, d.Name())
		if stat, err := os.Stat(plugpath); err == nil && stat.IsDir() {
			srcs, _ := ioutil.ReadDir(plugpath)
			p := new(Plugin)
			p.Name = d.Name()
			p.DirName = d.Name()
			for _, f := range srcs {
				if strings.HasSuffix(f.Name(), ".lua") {
					p.Srcs = append(p.Srcs, realFile(filepath.Join(plugdir, d.Name(), f.Name())))
				} else if strings.HasSuffix(f.Name(), ".json") {
					data, err := ioutil.ReadFile(filepath.Join(plugdir, d.Name(), f.Name()))
					if err != nil {
						continue
					}
					p.Info, err = NewPluginInfo(data)
					if err != nil {
						continue
					}
					p.Name = p.Info.Name
				}
			}
			if !isID(p.Name) || len(p.Srcs) <= 0 {
				log.Println(p.Name, "is not a plugin")
				continue
			}
			Plugins = append(Plugins, p)
		}
	}
	plugdir = filepath.Join("runtime", "plugins")
	if files, err := rt.AssetDir(plugdir); err == nil {
	outer:
		for _, d := range files {
			for _, p := range Plugins {
				if p.Name == d {
					log.Println(p.Name, "built-in plugin overridden by user-defined one")
					continue outer
				}
			}
			if srcs, err := rt.AssetDir(filepath.Join(plugdir, d)); err == nil {
				p := new(Plugin)
				p.Name = d
				p.DirName = d
				p.Default = true
				for _, f := range srcs {
					if strings.HasSuffix(f, ".lua") {
						p.Srcs = append(p.Srcs, assetFile(filepath.Join(plugdir, d, f)))
					} else if strings.HasSuffix(f, ".json") {
						data, err := rt.Asset(filepath.Join(plugdir, d, f))
						if err != nil {
							continue
						}
						p.Info, err = NewPluginInfo(data)
						if err != nil {
							continue
						}
						p.Name = p.Info.Name
					}
				}
				if !isID(p.Name) || len(p.Srcs) <= 0 {
					log.Println(p.Name, "is not a plugin")
					continue
				}
				Plugins = append(Plugins, p)
			}
		}
	}
}
func PluginReadRuntimeFile(fileType RTFiletype, name string) string {
	if file := FindRuntimeFile(fileType, name); file != nil {
		if data, err := file.Data(); err == nil {
			return string(data)
		}
	}
	return ""
}
func PluginListRuntimeFiles(fileType RTFiletype) []string {
	files := ListRuntimeFiles(fileType)
	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.Name()
	}
	return result
}
func PluginAddRuntimeFile(plugin string, filetype RTFiletype, filePath string) error {
	pl := FindPlugin(plugin)
	if pl == nil {
		return errors.New("Plugin " + plugin + " does not exist")
	}
	pldir := pl.DirName
	fullpath := filepath.Join(ConfigDir, "plug", pldir, filePath)
	if _, err := os.Stat(fullpath); err == nil {
		AddRealRuntimeFile(filetype, realFile(fullpath))
	} else {
		fullpath = path.Join("runtime", "plugins", pldir, filePath)
		AddRuntimeFile(filetype, assetFile(fullpath))
	}
	return nil
}
func PluginAddRuntimeFilesFromDirectory(plugin string, filetype RTFiletype, directory, pattern string) error {
	pl := FindPlugin(plugin)
	if pl == nil {
		return errors.New("Plugin " + plugin + " does not exist")
	}
	pldir := pl.DirName
	fullpath := filepath.Join(ConfigDir, "plug", pldir, directory)
	if _, err := os.Stat(fullpath); err == nil {
		AddRuntimeFilesFromDirectory(filetype, fullpath, pattern)
	} else {
		fullpath = path.Join("runtime", "plugins", pldir, directory)
		AddRuntimeFilesFromAssets(filetype, fullpath, pattern)
	}
	return nil
}
func PluginAddRuntimeFileFromMemory(filetype RTFiletype, filename, data string) {
	AddRealRuntimeFile(filetype, memoryFile{filename, []byte(data)})
}