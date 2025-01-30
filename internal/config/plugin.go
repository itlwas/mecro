package config
import (
	"errors"
	"log"
	lua "github.com/yuin/gopher-lua"
	ulua "github.com/zyedidia/micro/v2/internal/lua"
)
var ErrNoSuchFunction = errors.New("No such function exists")
func LoadAllPlugins() error {
	var reterr error
	for _, p := range Plugins {
		err := p.Load()
		if err != nil {
			reterr = err
		}
	}
	return reterr
}
func RunPluginFn(fn string, args ...lua.LValue) error {
	var reterr error
	for _, p := range Plugins {
		if !p.IsLoaded() {
			continue
		}
		_, err := p.Call(fn, args...)
		if err != nil && err != ErrNoSuchFunction {
			reterr = errors.New("Plugin " + p.Name + ": " + err.Error())
		}
	}
	return reterr
}
func RunPluginFnBool(settings map[string]interface{}, fn string, args ...lua.LValue) (bool, error) {
	var reterr error
	retbool := true
	for _, p := range Plugins {
		if !p.IsLoaded() || (settings != nil && settings[p.Name] == false) {
			continue
		}
		val, err := p.Call(fn, args...)
		if err == ErrNoSuchFunction {
			continue
		}
		if err != nil {
			reterr = errors.New("Plugin " + p.Name + ": " + err.Error())
			continue
		}
		if v, ok := val.(lua.LBool); ok {
			retbool = retbool && bool(v)
		}
	}
	return retbool, reterr
}
type Plugin struct {
	DirName string
	Name    string
	Info    *PluginInfo
	Srcs    []RuntimeFile
	Loaded  bool
	Default bool
}
func (p *Plugin) IsLoaded() bool {
	if v, ok := GlobalSettings[p.Name]; ok {
		return v.(bool) && p.Loaded
	}
	return true
}
var Plugins []*Plugin
func (p *Plugin) Load() error {
	if v, ok := GlobalSettings[p.Name]; ok && !v.(bool) {
		return nil
	}
	for _, f := range p.Srcs {
		dat, err := f.Data()
		if err != nil {
			return err
		}
		err = ulua.LoadFile(p.Name, f.Name(), dat)
		if err != nil {
			return err
		}
	}
	p.Loaded = true
	RegisterCommonOption(p.Name, true)
	return nil
}
func (p *Plugin) Call(fn string, args ...lua.LValue) (lua.LValue, error) {
	plug := ulua.L.GetGlobal(p.Name)
	if plug == lua.LNil {
		log.Println("Plugin does not exist:", p.Name, "at", p.DirName, ":", p)
		return nil, nil
	}
	luafn := ulua.L.GetField(plug, fn)
	if luafn == lua.LNil {
		return nil, ErrNoSuchFunction
	}
	err := ulua.L.CallByParam(lua.P{
		Fn:      luafn,
		NRet:    1,
		Protect: true,
	}, args...)
	if err != nil {
		return nil, err
	}
	ret := ulua.L.Get(-1)
	ulua.L.Pop(1)
	return ret, nil
}
func FindPlugin(name string) *Plugin {
	var pl *Plugin
	for _, p := range Plugins {
		if !p.IsLoaded() {
			continue
		}
		if p.Name == name {
			pl = p
			break
		}
	}
	return pl
}
func FindAnyPlugin(name string) *Plugin {
	var pl *Plugin
	for _, p := range Plugins {
		if p.Name == name {
			pl = p
			break
		}
	}
	return pl
}