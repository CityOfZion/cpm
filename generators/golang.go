package generators

import (
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"os"
	"strings"
)

func GenerateGoSDK(cfg *GenerateCfg) error {
	goconfig := binding.NewConfig()
	goconfig.Manifest = cfg.Manifest
	goconfig.Hash = cfg.ContractHash

	err := os.Mkdir("golang", 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", cfg.Manifest.Name, err)
	}

	f, err := os.Create("golang/" + strings.ToLower(cfg.Manifest.Name) + ".go")
	if err != nil {
		return fmt.Errorf("can't create output file: %w", err)
	}
	defer f.Close()

	goconfig.Output = f

	err = binding.Generate(goconfig)
	if err != nil {
		return fmt.Errorf("error during generation: %w", err)
	}
	return nil
}
