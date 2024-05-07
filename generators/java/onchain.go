package java

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

const javaSrcTmpl = `
{{- define "METHOD" }}
    public native {{.ReturnType }} {{.NameABI}}({{range $index, $arg := .Arguments -}}
       {{- if ne $index 0}}, {{end}}
          {{- .Type}} {{.Name}}
       {{- end}});
{{- end -}}
package <REPLACE_ME>;

import io.neow3j.devpack.*;
import io.neow3j.devpack.contracts.ContractInterface;


public class {{ .ContractName }} extends ContractInterface {

    static final String scriptHash = "{{.Hash}}";

    public {{ .ContractName }}() {
       super(scriptHash);
    }

{{- range $m := .Methods}}
{{ template "METHOD" $m -}}
{{end}}
}
`

func generateOnchainSDK(cfg *generators.GenerateCfg) error {
	err := createJavaPackage(cfg)
	defer cfg.ContractOutput.Close()
	if err != nil {
		return err
	}

	cfg.MethodNameConverter = strcase.ToLowerCamel
	cfg.ParamTypeConverter = scTypeToJava
	ctr, err := generators.TemplateFromManifest(cfg)
	if err != nil {
		return fmt.Errorf("failed to parse manifest into contract template: %v", err)
	}
	ctr.Hash = strings.TrimPrefix(ctr.Hash, "0x")

	tmp, err := template.New("generate").Parse(javaSrcTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse Java source template: %v", err)
	}

	err = tmp.Execute(cfg.ContractOutput, ctr)
	if err != nil {
		return fmt.Errorf("failed to generate Java code using template: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}

	log.Infof("Created SDK for contract '%s' at %s with contract hash 0x%s", cfg.Manifest.Name, wd+"/"+cfg.SdkDestination, cfg.ContractHash.StringLE())

	return nil
}

func createJavaPackage(cfg *generators.GenerateCfg) error {
	dir := cfg.SdkDestination
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", dir, err)
	}

	filename := generators.UpperFirst(cfg.Manifest.Name)
	f, err := os.Create(fmt.Sprintf(dir+"%s.java", filename))
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create %s.java file: %w", filename, err)
	} else {
		cfg.ContractOutput = f
	}

	return nil
}

func scTypeToJava(typ smartcontract.ParamType) string {
	switch typ {
	case smartcontract.AnyType:
		return "Object"
	case smartcontract.BoolType:
		return "boolean"
	case smartcontract.InteropInterfaceType:
		return "Object"
	case smartcontract.IntegerType:
		return "int"
	case smartcontract.ByteArrayType:
		return "ByteString"
	case smartcontract.StringType:
		return "String"
	case smartcontract.Hash160Type:
		return "Hash160"
	case smartcontract.Hash256Type:
		return "Hash256"
	case smartcontract.PublicKeyType:
		return "ECPoint"
	case smartcontract.ArrayType:
		return "List<Object>"
	case smartcontract.MapType:
		return "Map<Object, Object>"
	case smartcontract.VoidType:
		return "void"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}
