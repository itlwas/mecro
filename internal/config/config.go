package config
import (
	"errors"
	"os"
	"path/filepath"
	homedir "github.com/mitchellh/go-homedir"
)
var ConfigDir string
func InitConfigDir(flagConfigDir string) error {
	var e error
	mecroHome := os.Getenv("MECRO_CONFIG_HOME")
	if mecroHome == "" {
		xdgHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgHome == "" {
			home, err := homedir.Dir()
			if err != nil {
				return errors.New("Error finding your home directory\nCan't load config files: " + err.Error())
			}
			xdgHome = filepath.Join(home, ".config")
		}
		mecroHome = filepath.Join(xdgHome, "mecro")
	}
	ConfigDir = mecroHome
	if len(flagConfigDir) > 0 {
		if _, err := os.Stat(flagConfigDir); os.IsNotExist(err) {
			e = errors.New("Error: " + flagConfigDir + " does not exist. Defaulting to " + ConfigDir + ".")
		} else {
			ConfigDir = flagConfigDir
			return nil
		}
	}
	err := os.MkdirAll(ConfigDir, os.ModePerm)
	if err != nil {
		return errors.New("Error creating configuration directory: " + err.Error())
	}
	return e
}