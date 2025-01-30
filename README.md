# <img src="runtime/mecro.svg" style="height: 1em; vertical-align: text-top; margin-right: 0.4em;"> **MECRO** [![Go](https://img.shields.io/badge/go-391A80)](https://go.dev) [![Lua](https://img.shields.io/badge/lua-391A80)](https://lua.org)

> Modeless terminal text editor with enhanced customization.

**Mecro** is fork of Macro/Micro, focused on improved theming, syntax support, and intuitive configuration.  
Maintains compatibility with Micro's ecosystem while adding unique features.

## Key Features
- **170+ languages** with syntax highlighting
- **60+ built-in colorschemes** (16, 256, truecolor) + custom theme engine
- **Built-in file browser** - opens automatically when launched without arguments
- Modeless editing with **familiar keybindings** (Ctrl+S/Ctrl+Q/Ctrl+E)
- Split windows and tabbed editing
- **JSON-configurable** settings and keybindings
- **Lua scripting** for advanced customization and plugins
- Context-aware commands (type `> help` with Ctrl+E)
- Mouse support, multiple cursors, and macros
- Per-buffer settings via glob patterns
- Integrated command runner (Ctrl+Space)

## Install & Build
Requires Go 1.16+:
```bash
git clone https://github.com/yourusername/mecro
cd mecro
make build
```

## Quick Start
- `mecro` (no arguments) to launch the file browser
- `mecro file.txt` to edit files directly
- **Save**: Ctrl+S | **Quit**: Ctrl+Q | **Commands**: Ctrl+E
- Explore features with `> help tutorial` in command mode
- Customize via `~/.config/mecro/settings.json` and `bindings.json`

## Documentation
- In-editor help: Type `> help` followed by topics like `keybindings`, `plugins`, or `colors`
- **For plugin licenses/development**, refer to upstream [Macro](https://github.com/shkschneider/macro)
- Micro upstream docs: [micro-editor.github.io](https://micro-editor.github.io)

## Notes
- **Linux clipboard**: Requires `xclip`/`xsel` (X11) or `wl-clipboard` (Wayland)
- Configuration examples in `runtime/help/tutorial.md`
- Report issues **only to this repository** - differences from upstream exist!

---

Lightweight • Hackable • Compatible with Micro plugins • Integrated File Explorer
