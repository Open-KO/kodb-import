package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFileName = "kodb-import-config.yaml"
)

var (
	// ConfigPath is where the application will load the configuration file from.
	// This path is relative to the working directory; not this source file.
	// This can be overridden via CLI argument -config
	ConfigPath = ""

	// configInstance implements a singleton for the GetConfig() function
	configInstance *KodbConfig
)

// KodbConfig is the structure that binds the values in the configuration file
type KodbConfig struct {
	DatabaseConfig DatabaseConfig `yaml:"databaseConfig"`
	GenConfig      GenConfig      `yaml:"genConfig"`
}

// DatabaseConfig contains the connection configuration for an MSSQL server instance
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Instance string `yaml:"instance"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// GenConfig contains the configuration used to generate/export our application databases
type GenConfig struct {
	SchemaDir string `yaml:"schemaDir"`
	// TODO:  When implementing multi-db, we'll make this into an array of loginDb, gameDb, and logDb
	//	to allow more flexible generation
	GameDbs []GenDbConfig `yaml:"gameDb"`
}

// GenDbConfig contains the configuration for an individual application database
type GenDbConfig struct {
	Name    string        `yaml:"name"`
	Schemas []string      `yaml:"schemas"`
	Logins  []LoginConfig `yaml:"logins"`
	Users   []UserConfig  `yaml:"users"`
}

// LoginConfig contains the configuration of a single database login credential
type LoginConfig struct {
	Name string `yaml:"name"`
	Pass string `yaml:"pass"`
}

// UserConfig contains the configuration of a single database user
type UserConfig struct {
	Name   string `yaml:"name"`
	Schema string `yaml:"schema"`
}

// GetConfig returns a singleton instance of KodbConfig containing the application's configuration.
// can throw panic if config cannot be loaded
func GetConfig() *KodbConfig {
	if configInstance == nil {
		loadConfig()
	}

	return configInstance
}

// loadConfig attempts to read the configuration file and unmarshal it to a KodbConfig struct
// can throw panic if the configuration cannot be loaded
func loadConfig() {
	// Failing to load config is one of the few areas where we'll do a panic instead of error handling
	if ConfigPath == "" {
		ConfigPath = DefaultConfigFileName
	}
	absPath, pErr := filepath.Abs(ConfigPath)
	if pErr != nil {
		log.Panic(fmt.Errorf("failed to parse path for config: %v", pErr))
	}

	yamlFile, err := os.ReadFile(absPath)
	if err != nil {
		log.Panic(fmt.Errorf("failed to read config.yaml: %v", err))
	}

	err = yaml.Unmarshal(yamlFile, &configInstance)
	if err != nil {
		log.Panic(fmt.Errorf("failed to parse config.yaml: %v", err))
	}

}
