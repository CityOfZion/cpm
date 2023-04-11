package generators

import (
	"fmt"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	log "github.com/sirupsen/logrus"
)

func GenerateGoSDK(cfg *GenerateCfg) error {
	goconfig := binding.NewConfig()
	goconfig.Manifest = cfg.Manifest
	goconfig.Hash = cfg.ContractHash

	dir := cfg.SdkDestination
	err := os.Mkdir(dir, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", dir, err)
	}

	f, err := os.Create(dir + strings.ToLower(cfg.Manifest.Name) + ".go")
	if err != nil {
		return fmt.Errorf("can't create output file: %w", err)
	}
	defer f.Close()

	goconfig.Output = f

	err = binding.Generate(goconfig)
	if err != nil {
		return fmt.Errorf("error during generation: %w", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	sdkLocation := wd + "/" + dir + strings.ToLower(cfg.Manifest.Name) + ".go"
	log.Infof("Created SDK for contract '%s' at %s with contract hash 0x%s", cfg.Manifest.Name, sdkLocation, cfg.ContractHash.StringLE())
	return nil
}
