package generators

import (
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"os"
	"strconv"
	"strings"
)

// Big chunks of code gracefully borrowed from neo-go <3 with some adjustments

type (
	GenerateCfg struct {
		Manifest       *manifest.Manifest
		ContractHash   util.Uint160
		ContractOutput *os.File
	}

	contractTmpl struct {
		ContractName string
		Imports      []string
		Hash         string
		Methods      []methodTmpl
	}

	methodTmpl struct {
		Name       string
		NameABI    string
		Comment    string
		Arguments  []paramTmpl
		ReturnType string
	}

	paramTmpl struct {
		Name string
		Type string
	}

	convertParam func(typ smartcontract.ParamType) string
)

func templateFromManifest(cfg *GenerateCfg, scTypeToLang convertParam) (contractTmpl, error) {
	ctr := contractTmpl{
		ContractName: upperFirst(cfg.Manifest.Name),
		Hash:         "0x" + cfg.ContractHash.StringLE(),
	}

	seen := make(map[string]bool)
	for _, method := range cfg.Manifest.ABI.Methods {
		seen[method.Name] = false
	}

	for _, method := range cfg.Manifest.ABI.Methods {
		if method.Name[0] == '_' {
			continue
		}

		name := method.Name
		if v, ok := seen[name]; !ok || v {
			suffix := strconv.Itoa(len(method.Parameters))
			for ; seen[name]; name = method.Name + suffix {
				suffix = "_" + suffix
			}
		}
		seen[name] = true

		mtd := methodTmpl{
			Name:    name,
			NameABI: method.Name,
			Comment: fmt.Sprintf("invokes `%s` method of contract.", method.Name),
		}

		for i := range method.Parameters {
			name := method.Parameters[i].Name
			if name == "" {
				name = fmt.Sprintf("arg%d", i)
			}

			var typeStr = scTypeToLang(method.Parameters[i].Type)

			mtd.Arguments = append(mtd.Arguments, paramTmpl{
				Name: name,
				Type: typeStr,
			})
		}
		mtd.ReturnType = scTypeToLang(method.ReturnType)
		ctr.Methods = append(ctr.Methods, mtd)
	}
	return ctr, nil
}

func upperFirst(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}
