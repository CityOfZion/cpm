package generators

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const OutputRoot = "cpm_out/"

// Big chunks of code gracefully borrowed from neo-go <3 with some adjustments

type (
	GenerateCfg struct {
		Manifest            *manifest.Manifest
		ContractHash        util.Uint160
		ContractOutput      *os.File
		ParamTypeConverter  convertParam
		MethodNameConverter func(s string) string
		SdkDestination      string
	}

	ContractTmpl struct {
		ContractName string
		Imports      []string
		Hash         string
		Methods      []methodTmpl
		Events       []eventTmpl
	}

	methodTmpl struct {
		Name          string
		NameABI       string
		Comment       string
		Safe          bool
		Arguments     []paramTmpl
		ReturnType    string
		ReturnTypeABI string
	}

	eventTmpl struct {
		Name      string
		NameABI   string
		Arguments []paramTmpl
	}

	paramTmpl struct {
		Name    string
		Type    string
		TypeABI string
	}

	convertParam func(typ smartcontract.ParamType) string
)

func TemplateFromManifest(cfg *GenerateCfg) (ContractTmpl, error) {
	ctr := ContractTmpl{
		ContractName: cleanContractName(cfg.Manifest.Name),
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
			Name:    cfg.MethodNameConverter(name),
			NameABI: method.Name,
			Comment: fmt.Sprintf("invokes `%s` method of contract.", method.Name),
			Safe:    method.Safe,
		}

		for i := range method.Parameters {
			name := method.Parameters[i].Name
			if name == "" {
				name = fmt.Sprintf("arg%d", i)
			}

			var typeStr = cfg.ParamTypeConverter(method.Parameters[i].Type)

			mtd.Arguments = append(mtd.Arguments, paramTmpl{
				Name:    name,
				Type:    typeStr,
				TypeABI: (smartcontract.ParamType).String(method.Parameters[i].Type),
			})
		}
		mtd.ReturnType = cfg.ParamTypeConverter(method.ReturnType)
		mtd.ReturnTypeABI = (smartcontract.ParamType).String(method.ReturnType)
		ctr.Methods = append(ctr.Methods, mtd)
	}

	for _, event := range cfg.Manifest.ABI.Events {
		name := event.Name

		evt := eventTmpl{
			Name: name,
		}

		for i := range event.Parameters {
			name := event.Parameters[i].Name
			if name == "" {
				name = fmt.Sprintf("arg%d", i)
			}

			var typeStr = cfg.ParamTypeConverter(event.Parameters[i].Type)

			evt.Arguments = append(evt.Arguments, paramTmpl{
				Name: name,
				Type: typeStr,
			})
		}
		ctr.Events = append(ctr.Events, evt)
	}

	return ctr, nil
}

func UpperFirst(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}

func cleanContractName(s string) string {
	return UpperFirst(regexp.MustCompile(`[\W]+`).ReplaceAllString(s, ""))
}
