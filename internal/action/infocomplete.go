package action
import (
	"bytes"
	"sort"
	"strings"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/micro/v2/pkg/highlight"
)
func CommandComplete(b *buffer.Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	input, argstart := b.GetArg()
	var suggestions []string
	for cmd := range commands {
		if strings.HasPrefix(cmd, input) {
			suggestions = append(suggestions, cmd)
		}
	}
	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}
	return completions, suggestions
}
func HelpComplete(b *buffer.Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	input, argstart := b.GetArg()
	var suggestions []string
	for _, file := range config.ListRuntimeFiles(config.RTHelp) {
		topic := file.Name()
		if strings.HasPrefix(topic, input) {
			suggestions = append(suggestions, topic)
		}
	}
	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}
	return completions, suggestions
}
func colorschemeComplete(input string) (string, []string) {
	var suggestions []string
	files := config.ListRuntimeFiles(config.RTColorscheme)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), input) {
			suggestions = append(suggestions, f.Name())
		}
	}
	var chosen string
	if len(suggestions) == 1 {
		chosen = suggestions[0]
	}
	return chosen, suggestions
}
func filetypeComplete(input string) (string, []string) {
	var suggestions []string
	for _, f := range config.ListRealRuntimeFiles(config.RTSyntax) {
		data, err := f.Data()
		if err != nil {
			continue
		}
		header, err := highlight.MakeHeaderYaml(data)
		if err != nil {
			continue
		}
		if header.FileType == "off" || header.FileType == "unknown" {
			continue
		}
		if strings.HasPrefix(header.FileType, input) {
			suggestions = append(suggestions, header.FileType)
		}
	}
headerLoop:
	for _, f := range config.ListRuntimeFiles(config.RTSyntaxHeader) {
		data, err := f.Data()
		if err != nil {
			continue
		}
		header, err := highlight.MakeHeader(data)
		if err != nil {
			continue
		}
		for _, v := range suggestions {
			if v == header.FileType {
				continue headerLoop
			}
		}
		if strings.HasPrefix(header.FileType, input) {
			suggestions = append(suggestions, header.FileType)
		}
	}
	if strings.HasPrefix("off", input) {
		suggestions = append(suggestions, "off")
	}
	var chosen string
	if len(suggestions) == 1 {
		chosen = suggestions[0]
	}
	return chosen, suggestions
}
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
func OptionComplete(b *buffer.Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	input, argstart := b.GetArg()
	var suggestions []string
	for option := range config.GlobalSettings {
		if strings.HasPrefix(option, input) {
			suggestions = append(suggestions, option)
		}
	}
	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}
	return completions, suggestions
}
func OptionValueComplete(b *buffer.Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	l := b.LineBytes(c.Y)
	l = util.SliceStart(l, c.X)
	input, argstart := b.GetArg()
	completeValue := false
	args := bytes.Split(l, []byte{' '})
	if len(args) >= 2 {
		for option := range config.GlobalSettings {
			if option == string(args[len(args)-2]) {
				completeValue = true
				break
			}
		}
	}
	if !completeValue {
		return OptionComplete(b)
	}
	inputOpt := string(args[len(args)-2])
	inputOpt = strings.TrimSpace(inputOpt)
	var suggestions []string
	var optionVal interface{}
	for k, option := range config.GlobalSettings {
		if k == inputOpt {
			optionVal = option
		}
	}
	switch optionVal.(type) {
	case bool:
		if strings.HasPrefix("on", input) {
			suggestions = append(suggestions, "on")
		} else if strings.HasPrefix("true", input) {
			suggestions = append(suggestions, "true")
		}
		if strings.HasPrefix("off", input) {
			suggestions = append(suggestions, "off")
		} else if strings.HasPrefix("false", input) {
			suggestions = append(suggestions, "false")
		}
	case string:
		switch inputOpt {
		case "colorscheme":
			_, suggestions = colorschemeComplete(input)
		case "filetype":
			_, suggestions = filetypeComplete(input)
		case "sucmd":
			if strings.HasPrefix("sudo", input) {
				suggestions = append(suggestions, "sudo")
			}
			if strings.HasPrefix("doas", input) {
				suggestions = append(suggestions, "doas")
			}
		default:
			if choices, ok := config.OptionChoices[inputOpt]; ok {
				for _, choice := range choices {
					if strings.HasPrefix(choice, input) {
						suggestions = append(suggestions, choice)
					}
				}
			}
		}
	}
	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}
	return completions, suggestions
}
func PluginCmdComplete(b *buffer.Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	input, argstart := b.GetArg()
	var suggestions []string
	for _, cmd := range PluginCmds {
		if strings.HasPrefix(cmd, input) {
			suggestions = append(suggestions, cmd)
		}
	}
	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}
	return completions, suggestions
}
func PluginComplete(b *buffer.Buffer) ([]string, []string) {
	c := b.GetActiveCursor()
	l := b.LineBytes(c.Y)
	l = util.SliceStart(l, c.X)
	input, argstart := b.GetArg()
	completeValue := false
	args := bytes.Split(l, []byte{' '})
	if len(args) >= 2 {
		for _, cmd := range PluginCmds {
			if cmd == string(args[len(args)-2]) {
				completeValue = true
				break
			}
		}
	}
	if !completeValue {
		return PluginCmdComplete(b)
	}
	var suggestions []string
	for _, pl := range config.Plugins {
		if strings.HasPrefix(pl.Name, input) {
			suggestions = append(suggestions, pl.Name)
		}
	}
	sort.Strings(suggestions)
	completions := make([]string, len(suggestions))
	for i := range suggestions {
		completions[i] = util.SliceEndStr(suggestions[i], c.X-argstart)
	}
	return completions, suggestions
}