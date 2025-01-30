package highlight
import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"gopkg.in/yaml.v2"
)
type Group uint8
var Groups map[string]Group
var numGroups Group
func (g Group) String() string {
	for k, v := range Groups {
		if v == g {
			return k
		}
	}
	return ""
}
type Def struct {
	*Header
	rules *rules
}
type Header struct {
	FileType       string
	FileNameRegex  *regexp.Regexp
	HeaderRegex    *regexp.Regexp
	SignatureRegex *regexp.Regexp
}
type HeaderYaml struct {
	FileType string `yaml:"filetype"`
	Detect   struct {
		FNameRegexStr     string `yaml:"filename"`
		HeaderRegexStr    string `yaml:"header"`
		SignatureRegexStr string `yaml:"signature"`
	} `yaml:"detect"`
}
type File struct {
	FileType string
	yamlSrc  map[interface{}]interface{}
}
type pattern struct {
	group Group
	regex *regexp.Regexp
}
type rules struct {
	regions  []*region
	patterns []*pattern
	includes []string
}
type region struct {
	group      Group
	limitGroup Group
	parent     *region
	start      *regexp.Regexp
	end        *regexp.Regexp
	skip       *regexp.Regexp
	rules      *rules
}
func init() {
	Groups = make(map[string]Group)
}
func MakeHeader(data []byte) (*Header, error) {
	lines := bytes.Split(data, []byte{'\n'})
	if len(lines) < 4 {
		return nil, errors.New("Header file has incorrect format")
	}
	header := new(Header)
	var err error
	header.FileType = string(lines[0])
	fnameRegexStr := string(lines[1])
	headerRegexStr := string(lines[2])
	signatureRegexStr := string(lines[3])
	if fnameRegexStr != "" {
		header.FileNameRegex, err = regexp.Compile(fnameRegexStr)
	}
	if err == nil && headerRegexStr != "" {
		header.HeaderRegex, err = regexp.Compile(headerRegexStr)
	}
	if err == nil && signatureRegexStr != "" {
		header.SignatureRegex, err = regexp.Compile(signatureRegexStr)
	}
	if err != nil {
		return nil, err
	}
	return header, nil
}
func MakeHeaderYaml(data []byte) (*Header, error) {
	var hdrYaml HeaderYaml
	err := yaml.Unmarshal(data, &hdrYaml)
	if err != nil {
		return nil, err
	}
	header := new(Header)
	header.FileType = hdrYaml.FileType
	if hdrYaml.Detect.FNameRegexStr != "" {
		header.FileNameRegex, err = regexp.Compile(hdrYaml.Detect.FNameRegexStr)
	}
	if err == nil && hdrYaml.Detect.HeaderRegexStr != "" {
		header.HeaderRegex, err = regexp.Compile(hdrYaml.Detect.HeaderRegexStr)
	}
	if err == nil && hdrYaml.Detect.SignatureRegexStr != "" {
		header.SignatureRegex, err = regexp.Compile(hdrYaml.Detect.SignatureRegexStr)
	}
	if err != nil {
		return nil, err
	}
	return header, nil
}
func (header *Header) MatchFileName(filename string) bool {
	if header.FileNameRegex != nil {
		return header.FileNameRegex.MatchString(filename)
	}
	return false
}
func (header *Header) MatchFileHeader(firstLine []byte) bool {
	if header.HeaderRegex != nil {
		return header.HeaderRegex.Match(firstLine)
	}
	return false
}
func (header *Header) HasFileSignature() bool {
	return header.SignatureRegex != nil
}
func (header *Header) MatchFileSignature(line []byte) bool {
	if header.SignatureRegex != nil {
		return header.SignatureRegex.Match(line)
	}
	return false
}
func ParseFile(input []byte) (f *File, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("pkg: %v", r)
			}
		}
	}()
	var rules map[interface{}]interface{}
	if err = yaml.Unmarshal(input, &rules); err != nil {
		return nil, err
	}
	f = new(File)
	f.yamlSrc = rules
	for k, v := range rules {
		if k == "filetype" {
			filetype := v.(string)
			if filetype == "" {
				return nil, errors.New("empty filetype")
			}
			f.FileType = filetype
			break
		}
	}
	if f.FileType == "" {
		return nil, errors.New("missing filetype")
	}
	return f, err
}
func ParseDef(f *File, header *Header) (s *Def, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("pkg: %v", r)
			}
		}
	}()
	src := f.yamlSrc
	s = new(Def)
	s.Header = header
	for k, v := range src {
		if k == "rules" {
			inputRules := v.([]interface{})
			rules, err := parseRules(inputRules, nil)
			if err != nil {
				return nil, err
			}
			s.rules = rules
		}
	}
	if s.rules == nil {
		s.rules = new(rules)
	}
	return s, err
}
func HasIncludes(d *Def) bool {
	hasIncludes := len(d.rules.includes) > 0
	for _, r := range d.rules.regions {
		hasIncludes = hasIncludes || hasIncludesInRegion(r)
	}
	return hasIncludes
}
func hasIncludesInRegion(region *region) bool {
	hasIncludes := len(region.rules.includes) > 0
	for _, r := range region.rules.regions {
		hasIncludes = hasIncludes || hasIncludesInRegion(r)
	}
	return hasIncludes
}
func GetIncludes(d *Def) []string {
	includes := d.rules.includes
	for _, r := range d.rules.regions {
		includes = append(includes, getIncludesInRegion(r)...)
	}
	return includes
}
func getIncludesInRegion(region *region) []string {
	includes := region.rules.includes
	for _, r := range region.rules.regions {
		includes = append(includes, getIncludesInRegion(r)...)
	}
	return includes
}
func ResolveIncludes(def *Def, files []*File) {
	resolveIncludesInDef(files, def)
}
func resolveIncludesInDef(files []*File, d *Def) {
	for _, lang := range d.rules.includes {
		for _, searchFile := range files {
			if lang == searchFile.FileType {
				searchDef, _ := ParseDef(searchFile, nil)
				d.rules.patterns = append(d.rules.patterns, searchDef.rules.patterns...)
				d.rules.regions = append(d.rules.regions, searchDef.rules.regions...)
			}
		}
	}
	for _, r := range d.rules.regions {
		resolveIncludesInRegion(files, r)
		r.parent = nil
	}
}
func resolveIncludesInRegion(files []*File, region *region) {
	for _, lang := range region.rules.includes {
		for _, searchFile := range files {
			if lang == searchFile.FileType {
				searchDef, _ := ParseDef(searchFile, nil)
				region.rules.patterns = append(region.rules.patterns, searchDef.rules.patterns...)
				region.rules.regions = append(region.rules.regions, searchDef.rules.regions...)
			}
		}
	}
	for _, r := range region.rules.regions {
		resolveIncludesInRegion(files, r)
		r.parent = region
	}
}
func parseRules(input []interface{}, curRegion *region) (ru *rules, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("pkg: %v", r)
			}
		}
	}()
	ru = new(rules)
	for _, v := range input {
		rule := v.(map[interface{}]interface{})
		for k, val := range rule {
			group := k
			switch object := val.(type) {
			case string:
				if k == "include" {
					ru.includes = append(ru.includes, object)
				} else {
					r, err := regexp.Compile(object)
					if err != nil {
						return nil, err
					}
					groupStr := group.(string)
					if _, ok := Groups[groupStr]; !ok {
						numGroups++
						Groups[groupStr] = numGroups
					}
					groupNum := Groups[groupStr]
					ru.patterns = append(ru.patterns, &pattern{groupNum, r})
				}
			case map[interface{}]interface{}:
				region, err := parseRegion(group.(string), object, curRegion)
				if err != nil {
					return nil, err
				}
				ru.regions = append(ru.regions, region)
			default:
				return nil, fmt.Errorf("Bad type %T", object)
			}
		}
	}
	return ru, nil
}
func parseRegion(group string, regionInfo map[interface{}]interface{}, prevRegion *region) (r *region, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("pkg: %v", r)
			}
		}
	}()
	r = new(region)
	if _, ok := Groups[group]; !ok {
		numGroups++
		Groups[group] = numGroups
	}
	groupNum := Groups[group]
	r.group = groupNum
	r.parent = prevRegion
	r.start, err = regexp.Compile(regionInfo["start"].(string))
	if err != nil {
		return nil, err
	}
	r.end, err = regexp.Compile(regionInfo["end"].(string))
	if err != nil {
		return nil, err
	}
	if _, ok := regionInfo["skip"]; ok {
		r.skip, err = regexp.Compile(regionInfo["skip"].(string))
		if err != nil {
			return nil, err
		}
	}
	if _, ok := regionInfo["limit-group"]; ok {
		groupStr := regionInfo["limit-group"].(string)
		if _, ok := Groups[groupStr]; !ok {
			numGroups++
			Groups[groupStr] = numGroups
		}
		groupNum := Groups[groupStr]
		r.limitGroup = groupNum
		if err != nil {
			return nil, err
		}
	} else {
		r.limitGroup = r.group
	}
	r.rules, err = parseRules(regionInfo["rules"].([]interface{}), r)
	if err != nil {
		return nil, err
	}
	return r, nil
}