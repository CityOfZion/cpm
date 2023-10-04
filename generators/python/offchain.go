package python

import (
	"cpm/generators"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	log "github.com/sirupsen/logrus"
)

const pythonOffChainSrcTmpl = `
{{- define "METHOD" }}
	def {{.Name}}(self{{range $index, $arg := .Arguments -}}
		, {{.Name}}: {{.Type}}
		{{- end}}) -> ContractMethodResult[{{if eq .ReturnTypeABI "InteropInterface" }}list{{ else }}{{ .ReturnType }}{{ end }}]:
		{{- range $index, $arg := .Arguments}}
		{{- if eq $arg.TypeABI "Hash160" }}
		{{.Name}} = _check_address_and_convert({{.Name}})
		{{- end}}
		{{- end}}
		script = (
			vm.ScriptBuilder()
			{{if .Arguments -}}
			{{if eq .ReturnTypeABI "InteropInterface" -}}
			.emit_contract_call_with_args_and_unwrap_iterator
			{{- else -}}
			.emit_contract_call_with_args
			{{- end -}}
			(self.hash, "{{ .NameABI }}", [{{range $index, $arg := .Arguments}}
				{{- if ne $index 0}}, {{end}}{{- .Name}}
				{{- end}}])
			{{- else -}}
			{{if eq .ReturnTypeABI "InteropInterface" -}}
			.emit_contract_call_and_unwrap_iterator
			{{- else -}}
			.emit_contract_call
			{{- end -}}
			(self.hash, "{{ .NameABI }}")
			{{- end}}
			.to_array()
		)
		return ContractMethodResult(script, {{ MambaUnwrap .ReturnTypeABI }})
{{- end -}}
from neo3 import vm
from neo3.api import noderpc
from neo3.api.helpers import unwrap
from neo3.api.wrappers import GenericContract, ContractMethodResult, _check_address_and_convert
from neo3.core import types, cryptography, serialization
from neo3.wallet.types import NeoAddress


class {{ .ContractName }}(GenericContract):
	def __init__(self):
		super().__init__(types.UInt160.from_string("{{ .Hash }}"))

{{- range $m := .Methods}}
{{ template "METHOD" $m -}}
{{end}}
`

func generateOffchainSDK(cfg *generators.GenerateCfg) error {
	err := createOffChainPythonPackage(cfg)
	defer cfg.ContractOutput.Close()
	if err != nil {
		return err
	}

	cfg.MethodNameConverter = strcase.ToSnake
	cfg.ParamTypeConverter = scTypeToNeoMamba
	ctr, err := generators.TemplateFromManifest(cfg)
	if err != nil {
		return fmt.Errorf("failed to parse manifest into contract template: %v", err)
	}

	funcMap := template.FuncMap{
		"MambaUnwrap": mambaUnwrapTypes,
	}

	tmp, err := template.New("generate").Funcs(funcMap).Parse(pythonOffChainSrcTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse Python off chain source template: %v", err)
	}

	err = tmp.Execute(cfg.ContractOutput, ctr)
	if err != nil {
		return fmt.Errorf("failed to generate Python off chain SDK code using template: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	sdkLocation := wd + "/" + cfg.SdkDestination + generators.UpperFirst(cfg.Manifest.Name)
	log.Infof("Created off chain SDK for contract '%s' at %s with contract hash 0x%s", cfg.Manifest.Name, sdkLocation, cfg.ContractHash.StringLE())

	return nil
}

func createOffChainPythonPackage(cfg *generators.GenerateCfg) error {
	sdkDir := cfg.SdkDestination + strings.ReplaceAll(strings.ToLower(cfg.Manifest.Name), " ", "_")
	err := os.MkdirAll(sdkDir, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", sdkDir, err)
	}

	f, err := os.Create(sdkDir + "/__init__.py")
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create __init__.py file: %w", err)
	}
	f.Close()

	f, err = os.Create(sdkDir + "/contract_off_chain_sdk.py")
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create contract_off_chain_sdk.py file: %w", err)
	} else {
		cfg.ContractOutput = f
	}
	return nil
}

func scTypeToNeoMamba(typ smartcontract.ParamType) string {
	switch typ {
	case smartcontract.AnyType, smartcontract.InteropInterfaceType:
		return "noderpc.ContractParameter"
	case smartcontract.BoolType:
		return "bool"
	case smartcontract.IntegerType:
		return "int | types.BigInteger"
	case smartcontract.ByteArrayType:
		return "bytes | serialization.ISerializable"
	case smartcontract.StringType:
		return "str"
	case smartcontract.Hash160Type:
		return "types.UInt160 | NeoAddress"
	case smartcontract.Hash256Type:
		return "types.UInt256"
	case smartcontract.PublicKeyType:
		return "cryptography.ECPoint"
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

func mambaUnwrapTypes(typ string) string {
	switch typ {
	case "Any":
		return "unwrap.item"
	case "InteropInterface":
		return "unwrap.as_list"
	case "Boolean":
		return "unwrap.as_bool"
	case "Integer":
		return "unwrap.as_int"
	case "ByteArray":
		return "unwrap.as_bytes"
	case "String":
		return "unwrap.as_str"
	case "Hash160":
		return "unwrap.as_uint160"
	case "Hash256":
		return "unwrap.as_uint256"
	case "PublicKey":
		return "unwrap.as_public_key"
	case "Array":
		return "unwrap.as_list"
	case "Map":
		return "unwrap.as_dict"
	case "Void":
		return "unwrap.as_none"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}
