package explan

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	envExplanRoot  = "EXPLAN_ROOT"
	explanYAMLConf = "explan-config.yml"
)

// ConfigData is a container for ExPlan's configuration data, such as the
// SQL database schema signature
type ConfigData struct {
	// Logging instance
	log *log.Logger
	// SQLite schema signature (required)
	SQLSchemaSig string `yaml:"sqlSchemaSignature"`
	// Listen host/address and port
	ListenAddr string `yaml:"listenAddr"`
}

var (
	defaultConfig = ConfigData{
		nil,
		"**no signature",
		"localhost:3000",
	}
)

// GetExplanConfig reads ExPlan's configuration from a YAML-structured file and
// returns an ExplanConfigData structure with the contents.
func GetExplanConfig() (*ConfigData, error) {
	var explanYAML string
	var retval = defaultConfig

	retval.log = log.New(os.Stdout, "explan config: ", log.LstdFlags)

	if valExplanRoot, valid := os.LookupEnv(envExplanRoot); valid {
		explanYAML = valExplanRoot + string(os.PathSeparator) + explanYAMLConf
	} else {
		retval.log.Printf("Unset environment variable %s.", envExplanRoot)

		explanYAML = strings.Join([]string{"data", "config", explanYAMLConf}, string(os.PathSeparator))
		if _, err := os.Stat(explanYAML); errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	retval.log.Printf("Reading %s.", explanYAML)

	var yamlFile []byte
	var err error

	yamlFile, err = ioutil.ReadFile(explanYAML)
	if err != nil {
		retval.log.Printf("Could not read %s: %v", explanYAML, err)
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, &retval)
	if err != nil {
		retval.log.Printf("Error parsing %s: %v", explanYAML, err)
		return nil, err
	}

	return &retval, nil
}
