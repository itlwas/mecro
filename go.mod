module github.com/zyedidia/micro/v2
require (
github.com/blang/semver v3.5.1+incompatible
github.com/dustin/go-humanize v1.0.1
github.com/go-errors/errors v1.5.1
github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
github.com/mattn/go-isatty v0.0.20
github.com/mattn/go-runewidth v0.0.15
github.com/mitchellh/go-homedir v1.1.0
github.com/sergi/go-diff v1.3.1
github.com/stretchr/testify v1.4.0
github.com/yuin/gopher-lua v1.1.1
github.com/zyedidia/clipper v0.1.1
github.com/zyedidia/glob v0.0.0-20170209203856-dd4023a66dc3
github.com/zyedidia/json5 v0.0.0-20200102012142-2da050b1a98d
github.com/zyedidia/tcell/v2 v2.0.10
github.com/zyedidia/terminal v0.0.0-20230315200948-4b3bcf6dddef
golang.org/x/text v0.14.0
gopkg.in/yaml.v2 v2.4.0
layeh.com/gopher-luar v1.0.11
)
replace (
github.com/kballard/go-shellquote => github.com/zyedidia/go-shellquote v0.0.0-20200613203517-eccd813c0655
github.com/mattn/go-runewidth => github.com/zyedidia/go-runewidth v0.0.12
layeh.com/gopher-luar => github.com/layeh/gopher-luar v1.0.7
)
go 1.16