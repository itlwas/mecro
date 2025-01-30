package config
import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"github.com/zyedidia/glob"
	"github.com/zyedidia/json5"
	"github.com/zyedidia/micro/v2/internal/util"
	"golang.org/x/text/encoding/htmlindex"
)
type optionValidator func(string, interface{}) error
var optionValidators = map[string]optionValidator{
	"autosave":        validateNonNegativeValue,
	"clipboard":       validateChoice,
	"colorcolumn":     validateNonNegativeValue,
	"colorscheme":     validateColorscheme,
	"detectlimit":     validateNonNegativeValue,
	"encoding":        validateEncoding,
	"fileformat":      validateChoice,
	"matchbracestyle": validateChoice,
	"multiopen":       validateChoice,
	"reload":          validateChoice,
	"scrollmargin":    validateNonNegativeValue,
	"scrollspeed":     validateNonNegativeValue,
	"tabsize":         validatePositiveValue,
}
var OptionChoices = map[string][]string{
	"clipboard":       {"internal", "external", "terminal"},
	"fileformat":      {"unix", "dos"},
	"matchbracestyle": {"underline", "highlight"},
	"multiopen":       {"tab", "hsplit", "vsplit"},
	"reload":          {"prompt", "auto", "disabled"},
}
var defaultCommonSettings = map[string]interface{}{
	"autoindent":      true,
	"autosu":          false,
	"backup":          true,
	"backupdir":       "",
	"basename":        false,
	"colorcolumn":     float64(0),
	"cursorline":      true,
	"detectlimit":     float64(100),
	"diffgutter":      false,
	"encoding":        "utf-8",
	"eofnewline":      true,
	"fastdirty":       false,
	"fileformat":      defaultFileFormat(),
	"filetype":        "unknown",
	"hlsearch":        true,
	"hltaberrors":     false,
	"hltrailingws":    false,
	"incsearch":       true,
	"ignorecase":      true,
	"indentchar":      " ",
	"keepautoindent":  false,
	"matchbrace":      true,
	"matchbracestyle": "underline",
	"mkparents":       true,
	"permbackup":      false,
	"readonly":        false,
	"reload":          "prompt",
	"rmtrailingws":    false,
	"ruler":           true,
	"relativeruler":   false,
	"savecursor":      true,
	"saveundo":        true,
	"scrollbar":       true,
	"scrollmargin":    float64(3),
	"scrollspeed":     float64(2),
	"smartpaste":      true,
	"softwrap":        true,
	"splitbottom":     true,
	"splitright":      true,
	"statusformatl":   "$(filename) $(modified)($(line),$(col)) $(status.paste)| $(opt:filetype) | $(opt:fileformat) | $(opt:encoding)",
	"statusformatr":   "",
	"statusline":      true,
	"syntax":          true,
	"tabmovement":     false,
	"tabsize":         float64(4),
	"tabstospaces":    false,
	"useprimary":      true,
	"wordwrap":        true,
}
var DefaultGlobalOnlySettings = map[string]interface{}{
	"autosave":       float64(0),
	"clipboard":      "external",
	"colorscheme":    "catppuccin-mocha",
	"divchars":       "│—",
	"divreverse":     true,
	"fakecursor":     false,
	"helpsplit":      "hsplit",
	"infobar":        true,
	"keymenu":        false,
	"mouse":          true,
	"multiopen":      "vsplit",
	"parsecursor":    false,
	"paste":          false,
	"pluginchannels": []string{"https://raw.githubusercontent.com/micro-editor/plugin-channel/master/channel.json", "https://raw.githubusercontent.com/Neko-Box-Coder/unofficial-plugin-channel/stable/channel.json", "https://codeberg.org/micro-plugins/plugin-channel/raw/branch/main/channel.json"},
	"pluginrepos":    []string{},
	"savehistory":    true,
	"scrollbarchar":  "¦",
	"sucmd":          "sudo",
	"tabhighlight":   false,
	"tabreverse":     true,
	"xterm":          false,
}
var LocalSettings = []string{
	"filetype",
	"readonly",
}
var (
	ErrInvalidOption = errors.New("Invalid option")
	ErrInvalidValue  = errors.New("Invalid value")
	GlobalSettings map[string]interface{}
	parsedSettings     map[string]interface{}
	settingsParseError bool
	ModifiedSettings map[string]bool
	VolatileSettings map[string]bool
)
func init() {
	ModifiedSettings = make(map[string]bool)
	VolatileSettings = make(map[string]bool)
	parsedSettings = make(map[string]interface{})
}
func ReadSettings() error {
	filename := filepath.Join(ConfigDir, "settings.json")
	if _, e := os.Stat(filename); e == nil {
		input, err := ioutil.ReadFile(filename)
		if err != nil {
			settingsParseError = true
			return errors.New("Error reading settings.json file: " + err.Error())
		}
		if !strings.HasPrefix(string(input), "null") {
			err = json5.Unmarshal(input, &parsedSettings)
			if err != nil {
				settingsParseError = true
				return errors.New("Error reading settings.json: " + err.Error())
			}
			if v, ok := parsedSettings["autosave"]; ok {
				s, ok := v.(bool)
				if ok {
					if s {
						parsedSettings["autosave"] = 8.0
					} else {
						parsedSettings["autosave"] = 0.0
					}
				}
			}
		}
	}
	return nil
}
func verifySetting(option string, value reflect.Type, def reflect.Type) bool {
	var interfaceArr []interface{}
	switch option {
	case "pluginrepos", "pluginchannels":
		return value.AssignableTo(reflect.TypeOf(interfaceArr))
	default:
		return def.AssignableTo(value)
	}
}
func InitGlobalSettings() error {
	var err error
	GlobalSettings = DefaultGlobalSettings()
	for k, v := range parsedSettings {
		if !strings.HasPrefix(reflect.TypeOf(v).String(), "map") {
			if _, ok := GlobalSettings[k]; ok && !verifySetting(k, reflect.TypeOf(v), reflect.TypeOf(GlobalSettings[k])) {
				err = fmt.Errorf("Global Error: setting '%s' has incorrect type (%s), using default value: %v (%s)", k, reflect.TypeOf(v), GlobalSettings[k], reflect.TypeOf(GlobalSettings[k]))
				continue
			}
			GlobalSettings[k] = v
		}
	}
	return err
}
func InitLocalSettings(settings map[string]interface{}, path string) error {
	var parseError error
	for k, v := range parsedSettings {
		if strings.HasPrefix(reflect.TypeOf(v).String(), "map") {
			if strings.HasPrefix(k, "ft:") {
				if settings["filetype"].(string) == k[3:] {
					for k1, v1 := range v.(map[string]interface{}) {
						if _, ok := settings[k1]; ok && !verifySetting(k1, reflect.TypeOf(v1), reflect.TypeOf(settings[k1])) {
							parseError = fmt.Errorf("Error: setting '%s' has incorrect type (%s), using default value: %v (%s)", k, reflect.TypeOf(v1), settings[k1], reflect.TypeOf(settings[k1]))
							continue
						}
						settings[k1] = v1
					}
				}
			} else {
				g, err := glob.Compile(k)
				if err != nil {
					parseError = errors.New("Error with glob setting " + k + ": " + err.Error())
					continue
				}
				if g.MatchString(path) {
					for k1, v1 := range v.(map[string]interface{}) {
						if _, ok := settings[k1]; ok && !verifySetting(k1, reflect.TypeOf(v1), reflect.TypeOf(settings[k1])) {
							parseError = fmt.Errorf("Error: setting '%s' has incorrect type (%s), using default value: %v (%s)", k, reflect.TypeOf(v1), settings[k1], reflect.TypeOf(settings[k1]))
							continue
						}
						settings[k1] = v1
					}
				}
			}
		}
	}
	return parseError
}
func WriteSettings(filename string) error {
	if settingsParseError {
		return nil
	}
	var err error
	if _, e := os.Stat(ConfigDir); e == nil {
		defaults := DefaultGlobalSettings()
		for k, v := range parsedSettings {
			if !strings.HasPrefix(reflect.TypeOf(v).String(), "map") {
				cur, okcur := GlobalSettings[k]
				_, vol := VolatileSettings[k]
				if def, ok := defaults[k]; ok && okcur && !vol && reflect.DeepEqual(cur, def) {
					delete(parsedSettings, k)
				}
			}
		}
		for k, v := range GlobalSettings {
			if def, ok := defaults[k]; !ok || !reflect.DeepEqual(v, def) {
				if _, wr := ModifiedSettings[k]; wr {
					parsedSettings[k] = v
				}
			}
		}
		txt, _ := json.MarshalIndent(parsedSettings, "", "    ")
		err = ioutil.WriteFile(filename, append(txt, '\n'), 0644)
	}
	return err
}
func OverwriteSettings(filename string) error {
	settings := make(map[string]interface{})
	var err error
	if _, e := os.Stat(ConfigDir); e == nil {
		defaults := DefaultGlobalSettings()
		for k, v := range GlobalSettings {
			if def, ok := defaults[k]; !ok || !reflect.DeepEqual(v, def) {
				if _, wr := ModifiedSettings[k]; wr {
					settings[k] = v
				}
			}
		}
		txt, _ := json.MarshalIndent(settings, "", "    ")
		err = ioutil.WriteFile(filename, append(txt, '\n'), 0644)
	}
	return err
}
func RegisterCommonOptionPlug(pl string, name string, defaultvalue interface{}) error {
	return RegisterCommonOption(pl+"."+name, defaultvalue)
}
func RegisterGlobalOptionPlug(pl string, name string, defaultvalue interface{}) error {
	return RegisterGlobalOption(pl+"."+name, defaultvalue)
}
func RegisterCommonOption(name string, defaultvalue interface{}) error {
	if _, ok := GlobalSettings[name]; !ok {
		GlobalSettings[name] = defaultvalue
	}
	defaultCommonSettings[name] = defaultvalue
	return nil
}
func RegisterGlobalOption(name string, defaultvalue interface{}) error {
	if _, ok := GlobalSettings[name]; !ok {
		GlobalSettings[name] = defaultvalue
	}
	DefaultGlobalOnlySettings[name] = defaultvalue
	return nil
}
func GetGlobalOption(name string) interface{} {
	return GlobalSettings[name]
}
func defaultFileFormat() string {
	if runtime.GOOS == "windows" {
		return "dos"
	}
	return "unix"
}
func GetInfoBarOffset() int {
	offset := 0
	if GetGlobalOption("infobar").(bool) {
		offset++
	}
	if GetGlobalOption("keymenu").(bool) {
		offset += 2
	}
	return offset
}
func DefaultCommonSettings() map[string]interface{} {
	commonsettings := make(map[string]interface{})
	for k, v := range defaultCommonSettings {
		commonsettings[k] = v
	}
	return commonsettings
}
func DefaultGlobalSettings() map[string]interface{} {
	globalsettings := make(map[string]interface{})
	for k, v := range defaultCommonSettings {
		globalsettings[k] = v
	}
	for k, v := range DefaultGlobalOnlySettings {
		globalsettings[k] = v
	}
	return globalsettings
}
func DefaultAllSettings() map[string]interface{} {
	allsettings := make(map[string]interface{})
	for k, v := range defaultCommonSettings {
		allsettings[k] = v
	}
	for k, v := range DefaultGlobalOnlySettings {
		allsettings[k] = v
	}
	return allsettings
}
func GetNativeValue(option string, realValue interface{}, value string) (interface{}, error) {
	var native interface{}
	kind := reflect.TypeOf(realValue).Kind()
	if kind == reflect.Bool {
		b, err := util.ParseBool(value)
		if err != nil {
			return nil, ErrInvalidValue
		}
		native = b
	} else if kind == reflect.String {
		native = value
	} else if kind == reflect.Float64 {
		i, err := strconv.Atoi(value)
		if err != nil {
			return nil, ErrInvalidValue
		}
		native = float64(i)
	} else {
		return nil, ErrInvalidValue
	}
	if err := OptionIsValid(option, native); err != nil {
		return nil, err
	}
	return native, nil
}
func OptionIsValid(option string, value interface{}) error {
	if validator, ok := optionValidators[option]; ok {
		return validator(option, value)
	}
	return nil
}
func validatePositiveValue(option string, value interface{}) error {
	tabsize, ok := value.(float64)
	if !ok {
		return errors.New("Expected numeric type for " + option)
	}
	if tabsize < 1 {
		return errors.New(option + " must be greater than 0")
	}
	return nil
}
func validateNonNegativeValue(option string, value interface{}) error {
	nativeValue, ok := value.(float64)
	if !ok {
		return errors.New("Expected numeric type for " + option)
	}
	if nativeValue < 0 {
		return errors.New(option + " must be non-negative")
	}
	return nil
}
func validateChoice(option string, value interface{}) error {
	if choices, ok := OptionChoices[option]; ok {
		val, ok := value.(string)
		if !ok {
			return errors.New("Expected string type for " + option)
		}
		for _, v := range choices {
			if val == v {
				return nil
			}
		}
		choicesStr := strings.Join(choices, ", ")
		return errors.New(option + " must be one of: " + choicesStr)
	}
	return errors.New("Option has no pre-defined choices")
}
func validateColorscheme(option string, value interface{}) error {
	colorscheme, ok := value.(string)
	if !ok {
		return errors.New("Expected string type for colorscheme")
	}
	if !ColorschemeExists(colorscheme) {
		return errors.New(colorscheme + " is not a valid colorscheme")
	}
	return nil
}
func validateEncoding(option string, value interface{}) error {
	_, err := htmlindex.Get(value.(string))
	return err
}