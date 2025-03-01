package main
import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"syscall"
	"time"
	"github.com/go-errors/errors"
	isatty "github.com/mattn/go-isatty"
	lua "github.com/yuin/gopher-lua"
	"github.com/zyedidia/micro/v2/internal/action"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/clipboard"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/shell"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/tcell/v2"
)
var (
	flagVersion   = flag.Bool("version", false, "Show the version number and information")
	flagConfigDir = flag.String("config-dir", "", "Specify a custom location for the configuration directory")
	flagOptions   = flag.Bool("options", false, "Show all option help")
	flagDebug     = flag.Bool("debug", false, "Enable debug mode (prints debug info to ./log.txt)")
	flagProfile   = flag.Bool("profile", false, "Enable CPU profiling (writes profile info to ./mecro.prof)")
	flagPlugin    = flag.String("plugin", "", "Plugin command")
	flagClean     = flag.Bool("clean", false, "Clean configuration directory")
	optionFlags   map[string]*string
	sigterm chan os.Signal
	sighup  chan os.Signal
	timerChan chan func()
)
func InitFlags() {
	flag.Usage = func() {
		fmt.Println("Usage: mecro [OPTIONS] [FILE]...")
		fmt.Println("-clean")
		fmt.Println("    \tCleans the configuration directory")
		fmt.Println("-config-dir dir")
		fmt.Println("    \tSpecify a custom location for the configuration directory")
		fmt.Println("[FILE]:LINE:COL (if the `parsecursor` option is enabled)")
		fmt.Println("+LINE:COL")
		fmt.Println("    \tSpecify a line and column to start the cursor at when opening a buffer")
		fmt.Println("-options")
		fmt.Println("    \tShow all option help")
		fmt.Println("-debug")
		fmt.Println("    \tEnable debug mode (enables logging to ./log.txt)")
		fmt.Println("-profile")
		fmt.Println("    \tEnable CPU profiling (writes profile info to ./mecro.prof")
		fmt.Println("    \tso it can be analyzed later with \"go tool pprof mecro.prof\")")
		fmt.Println("-version")
		fmt.Println("    \tShow the version number and information")
		fmt.Print("\nMecro's plugins can be managed at the command line with the following commands.\n")
		fmt.Println("-plugin install [PLUGIN]...")
		fmt.Println("    \tInstall plugin(s)")
		fmt.Println("-plugin remove [PLUGIN]...")
		fmt.Println("    \tRemove plugin(s)")
		fmt.Println("-plugin update [PLUGIN]...")
		fmt.Println("    \tUpdate plugin(s) (if no argument is given, updates all plugins)")
		fmt.Println("-plugin search [PLUGIN]...")
		fmt.Println("    \tSearch for a plugin")
		fmt.Println("-plugin list")
		fmt.Println("    \tList installed plugins")
		fmt.Println("-plugin available")
		fmt.Println("    \tList available plugins")
		fmt.Print("\nMecro's options can also be set via command line arguments for quick\nadjustments. For real configuration, please use the settings.json\nfile (see 'help options').\n\n")
		fmt.Println("-option value")
		fmt.Println("    \tSet `option` to `value` for this session")
		fmt.Println("    \tFor example: `mecro -syntax off file.c`")
		fmt.Println("\nUse `mecro -options` to see the full list of configuration options")
	}
	optionFlags = make(map[string]*string)
	for k, v := range config.DefaultAllSettings() {
		optionFlags[k] = flag.String(k, "", fmt.Sprintf("The %s option. Default value: '%v'.", k, v))
	}
	flag.Parse()
	if *flagVersion {
		fmt.Println("Version:", util.Version)
		fmt.Println("Compiled on", util.CompileDate)
		os.Exit(0)
	}
	if *flagOptions {
		var keys []string
		m := config.DefaultAllSettings()
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			fmt.Printf("-%s value\n", k)
			fmt.Printf("    \tDefault value: '%v'\n", v)
		}
		os.Exit(0)
	}
	if util.Debug == "OFF" && *flagDebug {
		util.Debug = "ON"
	}
}
func DoPluginFlags() {
	if *flagClean || *flagPlugin != "" {
		config.LoadAllPlugins()
		if *flagPlugin != "" {
			args := flag.Args()
			config.PluginCommand(os.Stdout, *flagPlugin, args)
		} else if *flagClean {
			CleanConfig()
		}
		os.Exit(0)
	}
}
func LoadInput(args []string) []*buffer.Buffer {
	var filename string
	var input []byte
	var err error
	buffers := make([]*buffer.Buffer, 0, len(args))
	btype := buffer.BTDefault
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		btype = buffer.BTStdout
	}
	files := make([]string, 0, len(args))
	flagStartPos := buffer.Loc{-1, -1}
	flagr := regexp.MustCompile(`^\+(\d+)(?::(\d+))?$`)
	for _, a := range args {
		match := flagr.FindStringSubmatch(a)
		if len(match) == 3 && match[2] != "" {
			line, err := strconv.Atoi(match[1])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			col, err := strconv.Atoi(match[2])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			flagStartPos = buffer.Loc{col - 1, line - 1}
		} else if len(match) == 3 && match[2] == "" {
			line, err := strconv.Atoi(match[1])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			flagStartPos = buffer.Loc{0, line - 1}
		} else {
			files = append(files, a)
		}
	}
	if len(files) > 0 {
		for i := 0; i < len(files); i++ {
			buf, err := buffer.NewBufferFromFileAtLoc(files[i], btype, flagStartPos)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			buffers = append(buffers, buf)
		}
	} else if !isatty.IsTerminal(os.Stdin.Fd()) {
		input, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			screen.TermMessage("Error reading from stdin: ", err)
			input = []byte{}
		}
		buffers = append(buffers, buffer.NewBufferFromStringAtLoc(string(input), filename, btype, flagStartPos))
	} else {
		buffers = append(buffers, buffer.NewBufferFromStringAtLoc(string(input), filename, btype, flagStartPos))
	}
	return buffers
}
func main() {
	defer func() {
		if util.Stdout.Len() > 0 {
			fmt.Fprint(os.Stdout, util.Stdout.String())
		}
		os.Exit(0)
	}()
	var err error
	InitFlags()
	if *flagProfile {
		f, err := os.Create("mecro.prof")
		if err != nil {
			log.Fatal("error creating CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("error starting CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	InitLog()
	err = config.InitConfigDir(*flagConfigDir)
	if err != nil {
		screen.TermMessage(err)
	}
	config.InitRuntimeFiles(true)
	config.InitPlugins()
	err = config.ReadSettings()
	if err != nil {
		screen.TermMessage(err)
	}
	err = config.InitGlobalSettings()
	if err != nil {
		screen.TermMessage(err)
	}
	for k, v := range optionFlags {
		if *v != "" {
			nativeValue, err := config.GetNativeValue(k, config.DefaultAllSettings()[k], *v)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			config.GlobalSettings[k] = nativeValue
			config.VolatileSettings[k] = true
		}
	}
	DoPluginFlags()
	err = screen.Init()
	if err != nil {
		fmt.Println(err)
		fmt.Println("Fatal: Mecro could not initialize a Screen.")
		os.Exit(1)
	}
	m := clipboard.SetMethod(config.GetGlobalOption("clipboard").(string))
	clipErr := clipboard.Initialize(m)
	defer func() {
		if err := recover(); err != nil {
			if screen.Screen != nil {
				screen.Screen.Fini()
			}
			if e, ok := err.(*lua.ApiError); ok {
				fmt.Println("Lua API error:", e)
			} else {
				fmt.Println("Mecro encountered an error:", errors.Wrap(err, 2).ErrorStack())
			}
			for _, b := range buffer.OpenBuffers {
				b.Backup()
			}
			os.Exit(1)
		}
	}()
	err = config.LoadAllPlugins()
	if err != nil {
		screen.TermMessage(err)
	}
	action.InitBindings()
	action.InitCommands()
	err = config.InitColorscheme()
	if err != nil {
		screen.TermMessage(err)
	}
	err = config.RunPluginFn("preinit")
	if err != nil {
		screen.TermMessage(err)
	}
	action.InitGlobals()
	buffer.SetMessager(action.InfoBar)
	args := flag.Args()
	b := LoadInput(args)
	if len(b) == 0 {
		screen.Screen.Fini()
		runtime.Goexit()
	}
	action.InitTabs(b)
	err = config.RunPluginFn("init")
	if err != nil {
		screen.TermMessage(err)
	}
	err = config.RunPluginFn("postinit")
	if err != nil {
		screen.TermMessage(err)
	}
	if clipErr != nil {
		log.Println(clipErr, " or change 'clipboard' option")
	}
	if a := config.GetGlobalOption("autosave").(float64); a > 0 {
		config.SetAutoTime(int(a))
		config.StartAutoSave()
	}
	screen.Events = make(chan tcell.Event)
	sigterm = make(chan os.Signal, 1)
	sighup = make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT)
	signal.Notify(sighup, syscall.SIGHUP)
	timerChan = make(chan func())
	go func() {
		for {
			screen.Lock()
			e := screen.Screen.PollEvent()
			screen.Unlock()
			if e != nil {
				screen.Events <- e
			}
		}
	}()
	for len(screen.DrawChan()) > 0 {
		<-screen.DrawChan()
	}
	select {
	case event := <-screen.Events:
		action.Tabs.HandleEvent(event)
	case <-time.After(10 * time.Millisecond):
	}
	for {
		DoEvent()
	}
}
func DoEvent() {
	var event tcell.Event
	screen.Screen.Fill(' ', config.DefStyle)
	screen.Screen.HideCursor()
	action.Tabs.Display()
	for _, ep := range action.MainTab().Panes {
		ep.Display()
	}
	action.MainTab().Display()
	action.InfoBar.Display()
	screen.Screen.Show()
	select {
	case f := <-shell.Jobs:
		f.Function(f.Output, f.Args)
	case <-config.Autosave:
		for _, b := range buffer.OpenBuffers {
			b.AutoSave()
		}
	case <-shell.CloseTerms:
	case event = <-screen.Events:
	case <-screen.DrawChan():
		for len(screen.DrawChan()) > 0 {
			<-screen.DrawChan()
		}
	case f := <-timerChan:
		f()
	case <-sighup:
		for _, b := range buffer.OpenBuffers {
			if !b.Modified() {
				b.Fini()
			}
		}
		os.Exit(0)
	case <-sigterm:
		for _, b := range buffer.OpenBuffers {
			if !b.Modified() {
				b.Fini()
			}
		}
		if screen.Screen != nil {
			screen.Screen.Fini()
		}
		os.Exit(0)
	}
	if event == nil {
		return
	}
	if e, ok := event.(*tcell.EventError); ok {
		log.Println("tcell event error: ", e.Error())
		if e.Err() == io.EOF {
			for _, b := range buffer.OpenBuffers {
				if !b.Modified() {
					b.Fini()
				}
			}
			if screen.Screen != nil {
				screen.Screen.Fini()
			}
			os.Exit(0)
		}
		return
	}
	_, resize := event.(*tcell.EventResize)
	if resize {
		action.InfoBar.HandleEvent(event)
		action.Tabs.HandleEvent(event)
	} else if action.InfoBar.HasPrompt {
		action.InfoBar.HandleEvent(event)
	} else {
		action.Tabs.HandleEvent(event)
	}
}