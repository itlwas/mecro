package config
import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"github.com/blang/semver"
	lua "github.com/yuin/gopher-lua"
	"github.com/zyedidia/json5"
	ulua "github.com/zyedidia/micro/v2/internal/lua"
	"github.com/zyedidia/micro/v2/internal/util"
)
var (
	allPluginPackages PluginPackages
)
const CorePluginName = "mecro"
type PluginChannel string
type PluginChannels []PluginChannel
type PluginRepository string
type PluginPackage struct {
	Name        string
	Description string
	Author      string
	Tags        []string
	Versions    PluginVersions
}
type PluginPackages []*PluginPackage
type PluginVersion struct {
	pack    *PluginPackage
	Version semver.Version
	Url     string
	Require PluginDependencies
}
func (pv *PluginVersion) Pack() *PluginPackage {
	return pv.pack
}
type PluginVersions []*PluginVersion
type PluginDependency struct {
	Name  string
	Range semver.Range
}
type PluginDependencies []*PluginDependency
func (pp *PluginPackage) String() string {
	buf := new(bytes.Buffer)
	buf.WriteString("Plugin: ")
	buf.WriteString(pp.Name)
	buf.WriteRune('\n')
	if pp.Author != "" {
		buf.WriteString("Author: ")
		buf.WriteString(pp.Author)
		buf.WriteRune('\n')
	}
	if pp.Description != "" {
		buf.WriteRune('\n')
		buf.WriteString(pp.Description)
	}
	return buf.String()
}
func fetchAllSources(count int, fetcher func(i int) PluginPackages) PluginPackages {
	wgQuery := new(sync.WaitGroup)
	wgQuery.Add(count)
	results := make(chan PluginPackages)
	wgDone := new(sync.WaitGroup)
	wgDone.Add(1)
	var packages PluginPackages
	for i := 0; i < count; i++ {
		go func(i int) {
			results <- fetcher(i)
			wgQuery.Done()
		}(i)
	}
	go func() {
		packages = make(PluginPackages, 0)
		for res := range results {
			packages = append(packages, res...)
		}
		wgDone.Done()
	}()
	wgQuery.Wait()
	close(results)
	wgDone.Wait()
	return packages
}
func (pc PluginChannels) Fetch(out io.Writer) PluginPackages {
	return fetchAllSources(len(pc), func(i int) PluginPackages {
		return pc[i].Fetch(out)
	})
}
func (pc PluginChannel) Fetch(out io.Writer) PluginPackages {
	resp, err := http.Get(string(pc))
	if err != nil {
		fmt.Fprintln(out, "Failed to query plugin channel:\n", err)
		return PluginPackages{}
	}
	defer resp.Body.Close()
	decoder := json5.NewDecoder(resp.Body)
	var repositories []PluginRepository
	if err := decoder.Decode(&repositories); err != nil {
		fmt.Fprintln(out, "Failed to decode channel data:\n", err)
		return PluginPackages{}
	}
	return fetchAllSources(len(repositories), func(i int) PluginPackages {
		return repositories[i].Fetch(out)
	})
}
func (pr PluginRepository) Fetch(out io.Writer) PluginPackages {
	resp, err := http.Get(string(pr))
	if err != nil {
		fmt.Fprintln(out, "Failed to query plugin repository:\n", err)
		return PluginPackages{}
	}
	defer resp.Body.Close()
	decoder := json5.NewDecoder(resp.Body)
	var plugins PluginPackages
	if err := decoder.Decode(&plugins); err != nil {
		fmt.Fprintln(out, "Failed to decode repository data:\n", err)
		return PluginPackages{}
	}
	if len(plugins) > 0 {
		return PluginPackages{plugins[0]}
	}
	return nil
}
func (pv *PluginVersion) UnmarshalJSON(data []byte) error {
	var values struct {
		Version semver.Version
		Url     string
		Require map[string]string
	}
	if err := json5.Unmarshal(data, &values); err != nil {
		return err
	}
	pv.Version = values.Version
	pv.Url = values.Url
	pv.Require = make(PluginDependencies, 0)
	for k, v := range values.Require {
		if k != CorePluginName || !isUnknownCoreVersion() {
			if vRange, err := semver.ParseRange(v); err == nil {
				pv.Require = append(pv.Require, &PluginDependency{k, vRange})
			}
		}
	}
	return nil
}
func (pp *PluginPackage) UnmarshalJSON(data []byte) error {
	var values struct {
		Name        string
		Description string
		Author      string
		Tags        []string
		Versions    PluginVersions
	}
	if err := json5.Unmarshal(data, &values); err != nil {
		return err
	}
	pp.Name = values.Name
	pp.Description = values.Description
	pp.Author = values.Author
	pp.Tags = values.Tags
	pp.Versions = values.Versions
	for _, v := range pp.Versions {
		v.pack = pp
	}
	return nil
}
func GetAllPluginPackages(out io.Writer) PluginPackages {
	if allPluginPackages == nil {
		getOption := func(name string) []string {
			data := GetGlobalOption(name)
			if strs, ok := data.([]string); ok {
				return strs
			}
			if ifs, ok := data.([]interface{}); ok {
				result := make([]string, len(ifs))
				for i, urlIf := range ifs {
					if url, ok := urlIf.(string); ok {
						result[i] = url
					} else {
						return nil
					}
				}
				return result
			}
			return nil
		}
		channels := PluginChannels{}
		for _, url := range getOption("pluginchannels") {
			channels = append(channels, PluginChannel(url))
		}
		repos := []PluginRepository{}
		for _, url := range getOption("pluginrepos") {
			repos = append(repos, PluginRepository(url))
		}
		allPluginPackages = fetchAllSources(len(repos)+1, func(i int) PluginPackages {
			if i == 0 {
				return channels.Fetch(out)
			}
			return repos[i-1].Fetch(out)
		})
	}
	return allPluginPackages
}
func (pv PluginVersions) find(ppName string) *PluginVersion {
	for _, v := range pv {
		if v.pack.Name == ppName {
			return v
		}
	}
	return nil
}
func (pv PluginVersions) Len() int {
	return len(pv)
}
func (pv PluginVersions) Swap(i, j int) {
	pv[i], pv[j] = pv[j], pv[i]
}
func (pv PluginVersions) Less(i, j int) bool {
	return pv[i].Version.GT(pv[j].Version)
}
func (pp PluginPackage) Match(text string) bool {
	text = strings.ToLower(text)
	for _, t := range pp.Tags {
		if strings.ToLower(t) == text {
			return true
		}
	}
	if strings.Contains(strings.ToLower(pp.Name), text) {
		return true
	}
	if strings.Contains(strings.ToLower(pp.Description), text) {
		return true
	}
	return false
}
func (pp PluginPackage) IsInstallable(out io.Writer) error {
	_, err := GetAllPluginPackages(out).Resolve(GetInstalledVersions(true), PluginDependencies{
		&PluginDependency{
			Name:  pp.Name,
			Range: semver.Range(func(v semver.Version) bool { return true }),
		}})
	return err
}
func SearchPlugin(out io.Writer, texts []string) (plugins PluginPackages) {
	plugins = make(PluginPackages, 0)
pluginLoop:
	for _, pp := range GetAllPluginPackages(out) {
		for _, text := range texts {
			if !pp.Match(text) {
				continue pluginLoop
			}
		}
		if err := pp.IsInstallable(out); err == nil {
			plugins = append(plugins, pp)
		}
	}
	return
}
func isUnknownCoreVersion() bool {
	_, err := semver.ParseTolerant(util.Version)
	return err != nil
}
func newStaticPluginVersion(name, version string) *PluginVersion {
	vers, err := semver.ParseTolerant(version)
	if err != nil {
		if vers, err = semver.ParseTolerant("0.0.0-" + version); err != nil {
			vers = semver.MustParse("0.0.0-unknown")
		}
	}
	pl := &PluginPackage{
		Name: name,
	}
	pv := &PluginVersion{
		pack:    pl,
		Version: vers,
	}
	pl.Versions = PluginVersions{pv}
	return pv
}
func GetInstalledVersions(withCore bool) PluginVersions {
	result := PluginVersions{}
	if withCore {
		result = append(result, newStaticPluginVersion(CorePluginName, util.Version))
	}
	for _, p := range Plugins {
		if !p.IsLoaded() {
			continue
		}
		version := GetInstalledPluginVersion(p.Name)
		if pv := newStaticPluginVersion(p.Name, version); pv != nil {
			result = append(result, pv)
		}
	}
	return result
}
func GetInstalledPluginVersion(name string) string {
	plugin := ulua.L.GetGlobal(name)
	if plugin != lua.LNil {
		version := ulua.L.GetField(plugin, "VERSION")
		if str, ok := version.(lua.LString); ok {
			return string(str)
		}
	}
	return ""
}
func (pv *PluginVersion) DownloadAndInstall(out io.Writer) error {
	fmt.Fprintf(out, "Downloading %q (%s) from %q\n", pv.pack.Name, pv.Version, pv.Url)
	resp, err := http.Get(pv.Url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	zipbuf := bytes.NewReader(data)
	z, err := zip.NewReader(zipbuf, zipbuf.Size())
	if err != nil {
		return err
	}
	targetDir := filepath.Join(ConfigDir, "plug", pv.pack.Name)
	dirPerm := os.FileMode(0755)
	if err = os.MkdirAll(targetDir, dirPerm); err != nil {
		return err
	}
	var prefix string
	allPrefixed := false
	for i, f := range z.File {
		parts := strings.Split(f.Name, "/")
		if i == 0 {
			prefix = parts[0]
		} else if parts[0] != prefix {
			allPrefixed = false
			break
		} else {
			allPrefixed = true
		}
	}
	for _, f := range z.File {
		parts := strings.Split(f.Name, "/")
		if allPrefixed {
			parts = parts[1:]
		}
		targetName := filepath.Join(targetDir, filepath.Join(parts...))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetName, dirPerm); err != nil {
				return err
			}
		} else {
			basepath := filepath.Dir(targetName)
			if err := os.MkdirAll(basepath, dirPerm); err != nil {
				return err
			}
			content, err := f.Open()
			if err != nil {
				return err
			}
			defer content.Close()
			target, err := os.Create(targetName)
			if err != nil {
				return err
			}
			defer target.Close()
			if _, err = io.Copy(target, content); err != nil {
				return err
			}
		}
	}
	return nil
}
func (pl PluginPackages) Get(name string) *PluginPackage {
	for _, p := range pl {
		if p.Name == name {
			return p
		}
	}
	return nil
}
func (pl PluginPackages) GetAllVersions(name string) PluginVersions {
	result := make(PluginVersions, 0)
	p := pl.Get(name)
	if p != nil {
		result = append(result, p.Versions...)
	}
	return result
}
func (req PluginDependencies) Join(other PluginDependencies) PluginDependencies {
	m := make(map[string]*PluginDependency)
	for _, r := range req {
		m[r.Name] = r
	}
	for _, o := range other {
		cur, ok := m[o.Name]
		if ok {
			m[o.Name] = &PluginDependency{
				o.Name,
				o.Range.AND(cur.Range),
			}
		} else {
			m[o.Name] = o
		}
	}
	result := make(PluginDependencies, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}
func (all PluginPackages) Resolve(selectedVersions PluginVersions, open PluginDependencies) (PluginVersions, error) {
	if len(open) == 0 {
		return selectedVersions, nil
	}
	currentRequirement, stillOpen := open[0], open[1:]
	if currentRequirement != nil {
		if selVersion := selectedVersions.find(currentRequirement.Name); selVersion != nil {
			if currentRequirement.Range(selVersion.Version) {
				return all.Resolve(selectedVersions, stillOpen)
			}
			return nil, fmt.Errorf("unable to find a matching version for \"%s\"", currentRequirement.Name)
		}
		availableVersions := all.GetAllVersions(currentRequirement.Name)
		sort.Sort(availableVersions)
		for _, version := range availableVersions {
			if currentRequirement.Range(version.Version) {
				resolved, err := all.Resolve(append(selectedVersions, version), stillOpen.Join(version.Require))
				if err == nil {
					return resolved, nil
				}
			}
		}
		return nil, fmt.Errorf("unable to find a matching version for \"%s\"", currentRequirement.Name)
	}
	return selectedVersions, nil
}
func (pv PluginVersions) install(out io.Writer) {
	anyInstalled := false
	currentlyInstalled := GetInstalledVersions(true)
	for _, sel := range pv {
		if sel.pack.Name != CorePluginName {
			shouldInstall := true
			if pv := currentlyInstalled.find(sel.pack.Name); pv != nil {
				if pv.Version.NE(sel.Version) {
					fmt.Fprintln(out, "Uninstalling", sel.pack.Name)
					UninstallPlugin(out, sel.pack.Name)
				} else {
					shouldInstall = false
				}
			}
			if shouldInstall {
				if err := sel.DownloadAndInstall(out); err != nil {
					fmt.Fprintln(out, err)
					return
				}
				anyInstalled = true
			}
		}
	}
	if anyInstalled {
		fmt.Fprintln(out, "One or more plugins installed.")
	} else {
		fmt.Fprintln(out, "Nothing to install / update")
	}
}
func UninstallPlugin(out io.Writer, name string) {
	for _, p := range Plugins {
		if !p.IsLoaded() {
			continue
		}
		if p.Name == name {
			p.Loaded = false
			if err := os.RemoveAll(filepath.Join(ConfigDir, "plug", p.DirName)); err != nil {
				fmt.Fprintln(out, err)
				return
			}
			break
		}
	}
}
func (pl PluginPackage) Install(out io.Writer) {
	selected, err := GetAllPluginPackages(out).Resolve(GetInstalledVersions(true), PluginDependencies{
		&PluginDependency{
			Name:  pl.Name,
			Range: semver.Range(func(v semver.Version) bool { return true }),
		}})
	if err != nil {
		fmt.Fprintln(out, err)
		return
	}
	selected.install(out)
}
func UpdatePlugins(out io.Writer, plugins []string) {
	if len(plugins) == 0 {
		for _, p := range Plugins {
			if !p.IsLoaded() || p.Default {
				continue
			}
			plugins = append(plugins, p.Name)
		}
	}
	fmt.Fprintln(out, "Checking for plugin updates")
	mecroVersion := PluginVersions{
		newStaticPluginVersion(CorePluginName, util.Version),
	}
	var updates = make(PluginDependencies, 0)
	for _, name := range plugins {
		pv := GetInstalledPluginVersion(name)
		r, err := semver.ParseRange(">=" + pv)
		if err == nil {
			updates = append(updates, &PluginDependency{
				Name:  name,
				Range: r,
			})
		}
	}
	selected, err := GetAllPluginPackages(out).Resolve(mecroVersion, updates)
	if err != nil {
		fmt.Fprintln(out, err)
		return
	}
	selected.install(out)
}
func PluginCommand(out io.Writer, cmd string, args []string) {
	switch cmd {
	case "install":
		installedVersions := GetInstalledVersions(false)
		for _, plugin := range args {
			pp := GetAllPluginPackages(out).Get(plugin)
			if pp == nil {
				fmt.Fprintln(out, "Unknown plugin \""+plugin+"\"")
			} else if err := pp.IsInstallable(out); err != nil {
				fmt.Fprintln(out, "Error installing ", plugin, ": ", err)
			} else {
				for _, installed := range installedVersions {
					if pp.Name == installed.Pack().Name {
						if pp.Versions[0].Version.Compare(installed.Version) == 1 {
							fmt.Fprintln(out, pp.Name, " is already installed but out-of-date: use 'plugin update ", pp.Name, "' to update")
						} else {
							fmt.Fprintln(out, pp.Name, " is already installed")
						}
					}
				}
				pp.Install(out)
			}
		}
	case "remove":
		removed := ""
		for _, plugin := range args {
			for _, p := range Plugins {
				if p.Name == plugin && p.Default {
					fmt.Fprintln(out, "Default plugins cannot be removed, but can be disabled via settings.")
					continue
				}
				if p.Name == plugin {
					UninstallPlugin(out, plugin)
					removed += plugin + " "
					continue
				}
			}
		}
		if removed != "" {
			fmt.Fprintln(out, "Removed ", removed)
		} else {
			fmt.Fprintln(out, "No plugins removed")
		}
	case "update":
		UpdatePlugins(out, args)
	case "list":
		plugins := GetInstalledVersions(false)
		fmt.Fprintln(out, "The following plugins are currently installed:")
		for _, p := range plugins {
			fmt.Fprintf(out, "%s (%s)\n", p.Pack().Name, p.Version)
		}
	case "search":
		plugins := SearchPlugin(out, args)
		fmt.Fprintln(out, len(plugins), " plugins found")
		for _, p := range plugins {
			fmt.Fprintln(out, "----------------")
			fmt.Fprintln(out, p.String())
		}
		fmt.Fprintln(out, "----------------")
	case "available":
		packages := GetAllPluginPackages(out)
		fmt.Fprintln(out, "Available Plugins:")
		for _, pkg := range packages {
			fmt.Fprintln(out, pkg.Name)
		}
	default:
		fmt.Fprintln(out, "Invalid plugin command")
	}
}