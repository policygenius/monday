package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	// Filename is the name single YAML configuration file name
	Filename = "bifrost.yaml"
	// MultipleFilenamePattern is the name pattern for multiple YAML configuration files
	MultipleFilenamePattern = "bifrost*.yaml"
)

var (
	defaultConfigPath = os.Getenv("HOME")

	// Filepath is the path of the YAML configuration files when you
	// just have a single config file
	Filepath string

	// MultipleFilepath is the path of the YAML configuration files when
	// you define multiple config files
	MultipleFilepath string
)

func init() {
	setConfigFilePaths()
}

// Load method loads the configuration from the YAML configuration file
func Load() (*Config, error) {
	files := FindMultipleConfigFiles()

	if len(files) > 0 {
		err := createConfigFromMultiple(files)
		if err != nil {
			return nil, err
		}
	}

	err := CheckConfigFileExists()
	if err != nil {
		return nil, err
	}

	// Check for multiple config files
	var conf Config

	file, err := ioutil.ReadFile(Filepath)
	if err != nil {
		log.Printf("Error while reading config file: #%v", err)
	}

	err = yaml.Unmarshal(file, &conf)
	if err != nil {
		panic(fmt.Sprintf("An error has occured while reading configuration file:\n%v", err))
	}

	// Override GOPATH environment variable if defined in configuration
	if conf.GoPath != "" {
		os.Setenv("GOPATH", conf.GoPath)
	}

	// Set Kubeconfig filepath if defined in configuration
	if conf.KubeConfig != "" {
		os.Setenv("MONDAY_KUBE_CONFIG", conf.KubeConfig)
	}

	return &conf, nil
}

// FindMultipleConfigFiles finds if multiple configuration files has been created
func FindMultipleConfigFiles() []string {
	matches, _ := filepath.Glob(MultipleFilepath)

	for i, match := range matches {
		if strings.Contains(match, Filepath) {
			matches = append(matches[:i], matches[i+1:]...)
		}
	}

	return matches
}

// Merge multiple configuration files into a single one
func createConfigFromMultiple(matches []string) error {
	configFile, err := os.Create(Filepath)
	if err != nil {
		return err
	}
	defer configFile.Close()

	added := 0
	for _, match := range matches {
		file, err := ioutil.ReadFile(match)
		if err != nil {
			continue
		}

		configFile.Write(file)
		added++
	}

	if added == 0 {
		return errors.New("Unable to process any configuration file")
	}

	return nil
}

// CheckConfigFileExists ensures that config file is present before going further
func CheckConfigFileExists() error {
	if _, err := os.Stat(Filepath); os.IsNotExist(err) {
		return errors.New("Configuration file not found. If you run for the first time, please use 'init' command")
	}

	return nil
}

// GetProjectNames returns the project names as a list
func (c *Config) GetProjectNames() []string {
	list := make([]string, 0)

	for _, project := range c.Projects {
		list = append(list, project.Name)
	}

	return list
}

// GetProjectByName returns a project configuration from its name
func (c *Config) GetProjectByName(name string) (*Project, error) {
	for _, project := range c.Projects {
		if project.Name == name {
			return project, nil
		}
	}

	return nil, fmt.Errorf("Unable to find project name '%s' in the configuration", name)
}

func getConfigPath() string {
	if value := os.Getenv("BIFROST_CONFIG_PATH"); value != "" {
		return value
	}

	return defaultConfigPath
}

func setConfigFilePaths() {
	Filepath = fmt.Sprintf("%s/%s", getConfigPath(), Filename)
	MultipleFilepath = fmt.Sprintf("%s/%s", getConfigPath(), MultipleFilenamePattern)
}
