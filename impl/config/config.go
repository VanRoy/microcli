package config

import (
	"bufio"
	"errors"
	"log"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/zalando/go-keyring"
)

const CONFIG_FILE = ".microbox/config.toml"
const KEYCHAIN_APP_PREFIX = "Microbox - "

var Options = GlobalOptions{
	Interactive: true,
	Verbose:     true,
}

type GlobalOptions struct {
	Verbose     bool
	Interactive bool
}

type Config struct {
	Git        GitConfig
	Initializr InitializrConfig
}

type GitConfig struct {
	Type                    string
	BaseUrl                 string
	PrivateToken            string
	GroupIds                []string
	IncludeArchivedProjects bool
	NormalizeName           bool
	CloneProtocol           string
	UseTokenForOperation    bool
}

type InitializrConfig struct {
	Url string
}

func Exist() (bool, error) {

	configFile, err := getConfigFile()
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func Load() (*Config, error) {

	if exist, err := Exist(); !exist {
		return nil, err
	}

	configFile, _ := getConfigFile()

	var conf Config

	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		return nil, err
	}

	// Retrieve password from keychain
	if len(conf.Git.PrivateToken) == 0 {
		conf.Git.PrivateToken = getPassword(conf.Git)
	}

	// Default to SSH clone protocol
	if len(conf.Git.CloneProtocol) == 0 {
		conf.Git.CloneProtocol = "ssh"
	}

	return &conf, nil
}

func Save(config Config) {

	configFile, _ := getConfigFile()

	dir := filepath.Dir(configFile)
	err := os.Mkdir(dir, 0755)
	if err != nil {
		panic(err)
	}

	file, _ := os.Create(configFile)

	defer closeFile(file)

	writer := bufio.NewWriter(file)

	// Save password on keychain
	if len(config.Git.PrivateToken) != 0 {
		savePassword(config.Git)
		//Cleanup password to avoid saving in FS
		config.Git.PrivateToken = ""
	}

	writeError := toml.NewEncoder(writer).Encode(config)
	if writeError != nil {
		panic(writeError)
	}
}

func getConfigFile() (string, error) {

	currentDir, err := os.Getwd()
	if err != nil {
		return "", errors.New("cannot retreive current dir")
	}

	return filepath.Join(currentDir, CONFIG_FILE), nil
}

func getServiceName(conf GitConfig) string {
	gitUrl, err := url.Parse(conf.BaseUrl)
	if err != nil {
		log.Fatalf("Cannot parse URL in GIT configuration : %s", err.Error())
		os.Exit(1)
	}

	return KEYCHAIN_APP_PREFIX + strings.ToLower(gitUrl.Host)
}

func getPassword(conf GitConfig) string {

	currentUser, _ := user.Current()

	secret, err := keyring.Get(getServiceName(conf), currentUser.Username)
	if err != nil {
		log.Fatalf("Cannot retrieve password from keychain : %s", err.Error())
		os.Exit(1)
	}

	return secret

}

func savePassword(conf GitConfig) {

	currentUser, _ := user.Current()

	err := keyring.Set(getServiceName(conf), currentUser.Username, conf.PrivateToken)
	if err != nil {
		log.Fatalf("Cannot store password on keychain : %s", err.Error())
		os.Exit(1)
	}

}

func closeFile(f *os.File) {
	err := f.Close()
	if err != nil {
		panic(err)
	}
}
