package abossworked

/* "scooter me fecit"

Copyright 2022 B. Scott Michel

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program. If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/securecookie"
	"gopkg.in/yaml.v3"
)

const (
	// The name of the worked YAML configuration file
	workedYAMLConf = "worked-config.yml"
)

// ConfigData is a container for abossworked's configuration data. It is tied to the
// worked-config.yml YAML template.
type ConfigData struct {
	// Logging instance
	ConfigLog *log.Logger
	// Defaults to the current directory in which this worked example operates.
	WorkedRoot string
	// ConfigDataDir is where we find the YAML files
	ConfigDataDir string
	// And the embedded YAML configuration
	yamlConfig
}

// seedData holds the various seeds and keys used for secure cookies
type seedData struct {
	// Session seed
	SessionSeed string `yaml:"session"`
	// Cookie seed
	CookieSeed string `yaml:"cookie"`
	// CSRF seed
	CSRFSeed string `yaml:"csrf"`
}

// featureData holds the authboss features that can be enabled and disabled.
type featureData struct {
	UseConfirm  bool `yaml:"confirm"`
	UseLock     bool `yaml:"lock"`
	UseRemember bool `yaml:"remember"`
}

// Debugging features
type debugFeatures struct {
	TemplateVars bool `yaml:"template_vars"`
}
type yamlConfig struct {
	// Listen host/address and port
	ListenAddr map[string]string `yaml:"listenAddr"`
	// Seed data for session identifiers and cookies:
	Seeds seedData `yaml:"seeds"`
	// Features:
	Features featureData `yaml:"features"`
	// Debugging
	Debugging debugFeatures `yaml:"debugging"`
}

var (
	// Default configuration
	defaultConfig = ConfigData{
		ConfigLog:     nil,
		WorkedRoot:    "",
		ConfigDataDir: "",
		yamlConfig: yamlConfig{
			ListenAddr: map[string]string{
				"host": "localhost",
				"port": "3000",
			},
			Seeds: seedData{
				SessionSeed: "",
				CookieSeed:  "",
				CSRFSeed:    "",
			},
			Features: featureData{
				UseConfirm:  true,
				UseLock:     true,
				UseRemember: true,
			},
			Debugging: debugFeatures{
				TemplateVars: true,
			},
		},
	}
)

// GetWorkedConfig reads the worked example's configuration from a YAML-structured file and
// returns an WorkedConfigData structure with the contents.
func GetWorkedConfig() (retval *ConfigData, err error) {
	// Create and copy. Wish there were a slicker way to copy default values into the newly
	// allocated ConfigData
	retval = new(ConfigData)
	*retval = defaultConfig

	retval.ConfigLog = log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)

	retval.WorkedRoot, err = os.Getwd()
	if err != nil {
		return nil, errors.New("unable to determine current directory, aborting")
	}

	retval.ConfigDataDir = strings.Join([]string{retval.WorkedRoot, "data", "config"}, string(os.PathSeparator))
	workedYAML := strings.Join([]string{retval.ConfigDataDir, workedYAMLConf}, string(os.PathSeparator))

	if _, err = os.Stat(workedYAML); errors.Is(err, os.ErrNotExist) {
		retval.ConfigLog.Printf("YAML file %s, not found.", workedYAML)
		retval.ConfigLog.Printf("No configuration found, aborting.")
		return nil, err
	}

	retval.ConfigLog.Printf("Reading %s.", workedYAML)

	var yamlFile []byte

	yamlFile, err = ioutil.ReadFile(workedYAML)
	if err != nil {
		retval.ConfigLog.Printf("Could not read %s: %v", workedYAML, err)
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, &retval.yamlConfig)
	if err != nil {
		retval.ConfigLog.Printf("Error parsing %s: %v", workedYAML, err)
		return nil, err
	}

	// Sanity check the seeds:
	if len(retval.yamlConfig.Seeds.SessionSeed) == 0 {
		return nil, errors.New("missing session seed in configuration")
	}

	if len(retval.yamlConfig.Seeds.CookieSeed) == 0 {
		return nil, errors.New("missing cookie seed in configuration")
	}

	if len(retval.yamlConfig.Seeds.CSRFSeed) == 0 {
		return nil, errors.New("missing CSRF seed in configuration")
	}

	return retval, nil
}

// GenerateWorkedConfig is a utility function that generates the session, cookie and CSRF seeds and
// writes the YAML configuration file. It will not overwrite an existing configuration.
func GenerateWorkedConfig() {
	var err error

	// Create and copy. Wish there were a slicker way to copy default values into the newly
	// allocated ConfigData
	retval := defaultConfig

	retval.ConfigLog = log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)

	retval.WorkedRoot, err = os.Getwd()
	if err != nil {
		retval.ConfigLog.Print("unable to determine current directory, aborting")
		return
	}

	for foundIt := false; !foundIt; {
		retval.ConfigDataDir = strings.Join([]string{retval.WorkedRoot, "data", "config"}, string(os.PathSeparator))
		if _, err = os.Stat(retval.ConfigDataDir); err != nil {
			newWorkedRoot := filepath.Dir(retval.WorkedRoot)
			if len(newWorkedRoot) == 0 || newWorkedRoot == "." {
				retval.ConfigLog.Print("Could not find the data/config subdirectory. Aborting.")
			}

			retval.WorkedRoot = newWorkedRoot
		} else {
			foundIt = true
		}
	}

	workedYAML := strings.Join([]string{retval.ConfigDataDir, workedYAMLConf}, string(os.PathSeparator))

	if _, err = os.Stat(workedYAML); err == nil {
		retval.ConfigLog.Printf("YAML file %s exists.", workedYAML)
		retval.ConfigLog.Printf("Will not overwrite the existing configuration.")
		return
	}

	sessionSeed := securecookie.GenerateRandomKey(64)
	cookieSeed := securecookie.GenerateRandomKey(64)
	csrfSeed := securecookie.GenerateRandomKey(32)

	retval.ConfigLog.Printf("Generated session seed: %x\n", sessionSeed)
	retval.ConfigLog.Printf("Generated cookie seed:  %x\n", cookieSeed)
	retval.ConfigLog.Printf("Generated csrf seed:    %x\n", csrfSeed)

	retval.yamlConfig.Seeds.SessionSeed = base64.StdEncoding.EncodeToString(sessionSeed)
	retval.yamlConfig.Seeds.CookieSeed = base64.StdEncoding.EncodeToString(cookieSeed)
	retval.yamlConfig.Seeds.CSRFSeed = base64.StdEncoding.EncodeToString(csrfSeed)

	retval.ConfigLog.Printf("Writing %s.", workedYAML)

	yamlFile, err := yaml.Marshal(&retval.yamlConfig)
	if err != nil {
		retval.ConfigLog.Printf("Error marshalling YAML: %v", err)
		return
	}

	err = ioutil.WriteFile(workedYAML, yamlFile, 0666)
	if err != nil {
		retval.ConfigLog.Printf("Error writing file: %v", err)
	}
}

// HostPortString generates the "host[:port]" string for HTTP paths
func (cfg *ConfigData) HostPortString() string {
	retval := cfg.ListenAddr["host"]
	if port, valid := cfg.ListenAddr["port"]; valid {
		retval += ":" + port
	}

	return retval
}
