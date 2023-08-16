package main

import (
	"cpm/generators"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

//go:embed cpm.yaml.default
var defaultConfig []byte
var cfg *CPMConfig

type ContractConfig struct {
	Label               string          `yaml:"label"`
	ScriptHash          util.Uint160    `yaml:"script-hash"`
	SourceNetwork       *string         `yaml:"source-network,omitempty"`
	ContractGenerateSdk *bool           `yaml:"contract-generate-sdk,omitempty"`
	OnChain             *GenerateConfig `yaml:"on-chain,omitempty"`
	OffChain            *GenerateConfig `yaml:"off-chain,omitempty"`
}

type GenerateConfig struct {
	Languages       []string       `yaml:"languages"`
	SdkDestinations SdkDestination `yaml:"destinations"`
}

type SdkDestination struct {
	Csharp *string `yaml:"csharp,omitempty"`
	Golang *string `yaml:"go,omitempty"`
	Java   *string `yaml:"java,omitempty"`
	Python *string `yaml:"python,omitempty"`
}

type CPMConfig struct {
	Defaults struct {
		ContractSourceNetwork string          `yaml:"contract-source-network"`
		ContractDestination   string          `yaml:"contract-destination"`
		ContractGenerateSdk   bool            `yaml:"contract-generate-sdk"`
		OnChain               *GenerateConfig `yaml:"on-chain,omitempty"`
		OffChain              *GenerateConfig `yaml:"off-chain,omitempty"`
	} `yaml:"defaults"`
	Contracts []ContractConfig `yaml:"contracts"`
	Tools     struct {
		NeoExpress struct {
			CanGenerateSDK      bool    `yaml:"canGenerateSDK"`
			CanDownloadContract bool    `yaml:"canDownloadContract"`
			ExecutablePath      *string `yaml:"executable-path,omitempty"`
			ConfigPath          string  `yaml:"config-path"`
		} `yaml:"neo-express"`
	} `yaml:"tools"`
	Networks []struct {
		Label string   `yaml:"label"`
		Hosts []string `yaml:"hosts"`
	} `yaml:"networks"`
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

	yamlData, _ := ioutil.ReadAll(f)
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
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

func (c *CPMConfig) addContract(label string, scriptHash util.Uint160) {
	for _, c := range cfg.Contracts {
		if c.ScriptHash.Equals(scriptHash) {
			return
		}
	}
	cfg.Contracts = append(cfg.Contracts, ContractConfig{Label: label, ScriptHash: scriptHash})
	saveConfig()
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
	if c.Defaults.OnChain == nil {
		return generators.OutputRoot + forLanguage + "/"
	}

	defaultLocation := generators.OutputRoot + forLanguage + "/"
	switch forLanguage {
	case LANG_PYTHON:
		if path := c.Defaults.OnChain.SdkDestinations.Python; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	case LANG_GO:
		if path := c.Defaults.OnChain.SdkDestinations.Golang; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	case LANG_JAVA:
		if path := c.Defaults.OnChain.SdkDestinations.Java; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	case LANG_CSHARP:
		if path := c.Defaults.OnChain.SdkDestinations.Csharp; path != nil {
			return EnsureSuffix(*path)
		}
		return defaultLocation
	default:
		return defaultLocation
	}
}

func saveConfig() {
	f, err := os.Create(DEFAULT_CONFIG_FILE)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return
	}

	_, err = f.Write(data)
	if err != nil {
		log.Fatal(err)
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
