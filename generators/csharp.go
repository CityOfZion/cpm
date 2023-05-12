package generators

import (
	"fmt"
	"os"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	log "github.com/sirupsen/logrus"
)

const csharpSrcTmpl = `
{{- define "METHOD" }}
        public static {{.ReturnType}} {{.Name}}({{range $index, $arg := .Arguments -}}
       {{- if ne $index 0}}, {{end}}
          {{- .Type}} {{.Name}}
       {{- end}}) {
            return ({{.ReturnType}}) Contract.Call(ScriptHash, "{{.NameABI}}", CallFlags.All
{{- range $arg := .Arguments -}}, {{ .Name -}} {{ else }}, new object[0]{{end}}); 
{{- end -}}
using Neo;
using Neo.Cryptography.ECC;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using Neo.SmartContract.Framework.Attributes;

namespace cpm {
    public class {{ .ContractName }}  {

        [InitialValue("{{.Hash}}", ContractParameterType.Hash160)]
        static readonly UInt160 ScriptHash;

        {{- range $m := .Methods}}
        {{ template "METHOD" $m -}}
        {{end}}
   }
}
`

func GenerateCsharpSDK(cfg *GenerateCfg) error {
	err := createCsharpPackage(cfg)
	defer cfg.ContractOutput.Close()
	if err != nil {
		return err
	}

	cfg.MethodNameConverter = strcase.ToCamel
	cfg.ParamTypeConverter = scTypeToCsharp
	ctr, err := templateFromManifest(cfg)
	if err != nil {
		return err
	}

	tmp, err := template.New("generate").Parse(csharpSrcTmpl)
	if err != nil {
		return err
	}

	err = tmp.Execute(cfg.ContractOutput, ctr)
	if err != nil {
		log.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	sdkLocation := wd + "/" + cfg.SdkDestination + upperFirst(cfg.Manifest.Name) + ".cs"
	log.Infof("Created SDK for contract '%s' at %s with contract hash 0x%s", cfg.Manifest.Name, sdkLocation, cfg.ContractHash.StringLE())

	return nil
}

func createCsharpPackage(cfg *GenerateCfg) error {
	dir := cfg.SdkDestination
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", dir, err)
	}

	filename := upperFirst(cfg.Manifest.Name)
	f, err := os.Create(fmt.Sprintf(dir+"%s.cs", filename))
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create %s.cs file: %w", filename, err)
	} else {
		cfg.ContractOutput = f
	}

	return nil
}

func scTypeToCsharp(typ smartcontract.ParamType) string {
	switch typ {
	case smartcontract.AnyType:
		return "object"
	case smartcontract.BoolType:
		return "bool"
	case smartcontract.InteropInterfaceType:
		return "object"
	case smartcontract.IntegerType:
		return "BigInteger"
	case smartcontract.ByteArrayType:
		return "byte[]"
	case smartcontract.StringType:
		return "string"
	case smartcontract.Hash160Type:
		return "UInt160"
	case smartcontract.Hash256Type:
		return "UInt256"
	case smartcontract.PublicKeyType:
		return "ECPoint"
	case smartcontract.ArrayType:
		return "object[]"
	case smartcontract.MapType:
		return "Map<object, object>"
	case smartcontract.VoidType:
		return "void"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}
