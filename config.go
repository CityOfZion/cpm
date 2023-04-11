package main

import (
	"cpm/generators"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
)

//go:embed sampleconfig.json
var defaultConfig []byte
var cfg CPMConfig

type ContractConfig struct {
	Label               string           `json:"label"`
	ScriptHash          util.Uint160     `json:"script-hash"`
	SourceNetwork       *string          `json:"source-network,omitempty"`
	ContractGenerateSdk *bool            `json:"contract-generate-sdk,omitempty"`
	SdkDestinations     *SdkDestinations `json:"sdk-destinations"`
}

type SdkDestinations struct {
	OnChain SdkDestination `json:"onchain"`
	//OffChain SdkDestination `json:"offchain"`
}

type SdkDestination struct {
	Csharp *string `json:"csharp"`
	Golang *string `json:"golang"`
	Java   *string `json:"java"`
	Python *string `json:"python"`
}

type CPMConfig struct {
	Defaults struct {
		ContractSourceNetwork string           `json:"contract-source-network"`
		ContractDestination   string           `json:"contract-destination"`
		ContractGenerateSdk   bool             `json:"contract-generate-sdk"`
		SdkLanguage           string           `json:"sdk-language"`
		SdkDestinations       *SdkDestinations `json:"sdk-destinations"`
	} `json:"defaults"`
	Contracts []ContractConfig `json:"contracts"`
	Tools     struct {
		NeoExpress struct {
			CanGenerateSDK      bool    `json:"canGenerateSDK"`
			CanDownloadContract bool    `json:"canDownloadContract"`
			ExecutablePath      *string `json:"executable-path"`
			ConfigPath          string  `json:"config-path"`
		} `json:"neo-express"`
	} `json:"tools"`
	Networks []struct {
		Label string   `json:"label"`
		Hosts []string `json:"hosts"`
	} `json:"networks"`
}

func LoadConfig() {
	f, err := os.Open(DEFAULT_CONFIG_FILE)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Config file %s not found. Run `cpm init` to create a default config", DEFAULT_CONFIG_FILE)
		} else {
			log.Fatal(err)
		}
	}
	defer f.Close()

	jsonData, _ := ioutil.ReadAll(f)
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		log.Fatal(fmt.Errorf("failed to parse config file: %w", err))
	}

	// ensure all contract configs can be worked with directly
	for i, c := range cfg.Contracts {
		if c.SourceNetwork == nil {
			cfg.Contracts[i].SourceNetwork = &cfg.Defaults.ContractSourceNetwork
		}
		if c.ContractGenerateSdk == nil {
			cfg.Contracts[i].ContractGenerateSdk = &cfg.Defaults.ContractGenerateSdk
		}
	}
}

func CreateDefaultConfig() {
	if _, err := os.Stat(DEFAULT_CONFIG_FILE); os.IsNotExist(err) {
		err = ioutil.WriteFile(DEFAULT_CONFIG_FILE, defaultConfig, 0644)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Written %s\n", DEFAULT_CONFIG_FILE)
	} else {
		log.Fatalf("%s already exists", DEFAULT_CONFIG_FILE)
	}
}

func (c *CPMConfig) getHosts(networkLabel string) []string {
	for _, network := range c.Networks {
		if network.Label == networkLabel {
			return network.Hosts
		}
	}
	log.Fatalf("Could not find hosts for label: %s", networkLabel)
	return nil
}

func (c *CPMConfig) getSdkDestination(forLanguage string) string {
	if c.Defaults.SdkDestinations == nil {
		return generators.OutputRoot + forLanguage + "/"
	}

	defaultLocation := generators.OutputRoot + forLanguage + "/"
	switch forLanguage {
	case LANG_PYTHON:
		if path := c.Defaults.SdkDestinations.OnChain.Python; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	case LANG_GO:
		if path := c.Defaults.SdkDestinations.OnChain.Golang; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	case LANG_JAVA:
		if path := c.Defaults.SdkDestinations.OnChain.Java; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	case LANG_CSHARP:
		if path := c.Defaults.SdkDestinations.OnChain.Csharp; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	default:
		return defaultLocation
	}
}

type EnumValue struct {
	Enum     []string
	Default  string
	selected string
}

func (e *EnumValue) Set(value string) error {
	for _, enum := range e.Enum {
		if enum == value {
			e.selected = value
			return nil
		}
	}

	return fmt.Errorf("allowed values are %s", strings.Join(e.Enum, ", "))
}

func (e EnumValue) String() string {
	if e.selected == "" {
		return e.Default
	}
	return e.selected
}
