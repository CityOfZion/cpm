package generators

import (
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"text/template"
)

/*
	Big chunks of code gracefully borrowed from neo-go <3 with some adjustments

	Creates a Python SDK that can be easily used when writing smart contracts with neo3-boa.
	The output is a Python package. For example for a contract named `samplecontract` it results in the folder structure
		.
		├── samplecontract
		│   ├── __init__.py
		│   └── contract.py

	which can be used in your neo3-boa contract with

		from samplecontract import Samplecontract
		Samplecontract.func1()
*/

type (
	PythonGenerateCfg struct {
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
)

const srcTmpl = `
{{- define "METHOD" }}
    @staticmethod
    def {{.Name}}({{range $index, $arg := .Arguments -}}
       {{- if ne $index 0}}, {{end}}
          {{- .Name}}: {{.Type}}
       {{- end}}) -> {{if .ReturnType }}{{ .ReturnType }}: {{ else }} None: {{ end }}
        pass
{{- end -}}
from boa3.builtin.interop.contract import call_contract
from boa3.builtin.type import UInt160, UInt256, ECPoint
from boa3.builtin import contract
from typing import cast, Any


@contract('{{ .Hash }}')
class {{ .ContractName }}:
{{- range $m := .Methods}}
{{ template "METHOD" $m -}}
{{end}}`

func GeneratePythonSDK(cfg *PythonGenerateCfg) error {
	wd, err := os.Getwd()

	err = createSDKPackage(cfg)
	defer cfg.ContractOutput.Close()
	if err != nil {
		return err
	}

	wdContract, err := os.Getwd()
	if err != nil {
		return err
	}

	ctr, err := templateFromManifest(cfg)
	if err != nil {
		return err
	}

	tmp, err := template.New("generate").Parse(srcTmpl)
	if err != nil {
		return err
	}

	err = tmp.Execute(cfg.ContractOutput, ctr)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Created SDK for contract '%s' at %s with contract hash 0x%s", cfg.Manifest.Name, wdContract, cfg.ContractHash.StringLE())

	// change dir back to project root
	os.Chdir(wd)

	return nil
}

func templateFromManifest(cfg *PythonGenerateCfg) (contractTmpl, error) {
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

			var typeStr = scTypeToPython(method.Parameters[i].Type)

			mtd.Arguments = append(mtd.Arguments, paramTmpl{
				Name: name,
				Type: typeStr,
			})
		}
		mtd.ReturnType = scTypeToPython(method.ReturnType)
		ctr.Methods = append(ctr.Methods, mtd)
	}
	return ctr, nil
}

// create the Python package structure and set the ContractOutput to the open file handle
func createSDKPackage(cfg *PythonGenerateCfg) error {
	err := os.Mkdir(cfg.Manifest.Name, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", cfg.Manifest.Name, err)
	}

	_ = os.Chdir(cfg.Manifest.Name)

	f, err := os.Create("__init__.py")
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create __init__.py file: %w", err)
	} else {
		f.WriteString(fmt.Sprintf("from .contract import %s\n", upperFirst(cfg.Manifest.Name)))
		f.Close()
	}

	f, err = os.Create("contract.py")
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create contract.py file: %w", err)
	} else {
		cfg.ContractOutput = f
	}
	return nil
}

func scTypeToPython(typ smartcontract.ParamType) string {
	switch typ {
	case smartcontract.AnyType, smartcontract.InteropInterfaceType:
		return "Any"
	case smartcontract.BoolType:
		return "bool"
	case smartcontract.IntegerType:
		return "int"
	case smartcontract.ByteArrayType:
		return "bytes"
	case smartcontract.StringType:
		return "str"
	case smartcontract.Hash160Type:
		return "UInt160"
	case smartcontract.Hash256Type:
		return "UInt256"
	case smartcontract.PublicKeyType:
		return "ECPoint"
	case smartcontract.ArrayType:
		return "list"
	case smartcontract.MapType:
		return "dict"
	case smartcontract.VoidType:
		return "None"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}

func upperFirst(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}
