package java

import (
	"cpm/generators"
	"fmt"
	"os"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	log "github.com/sirupsen/logrus"
)

const javaOffChainSrcTmpl = `
{{- define "INVOKEMETHOD" }}
	public TransactionBuilder {{ .Name }}({{range $index, $arg := .Arguments -}}
			{{- if ne $index 0}}, {{end}}{{.Type}} {{.Name}}
		{{- end}}) {
		return smartContract.invokeFunction("{{ .NameABI }}"{{if not .Arguments -}} ); {{- else}},
			{{- $length := len .Arguments -}}
			{{- range $index, $arg := .Arguments}}
			{{ Neow3jWrapParameter $arg.TypeABI }}({{ .Name }}){{ if lt $index (Dec $length) }},{{ end }}
			{{- end}}
		);{{- end}}
	}
{{- end -}}
{{- define "TESTINVOKEMETHOD" }}
	public {{ Neow3jReturnType .ReturnType }} {{ if .Safe }}{{ .Name }}{{ else }}test{{ UpperFirst .Name }}{{ end }}({{range $index, $arg := .Arguments -}}
		{{.Type}} {{.Name}}, {{ end }}AccountSigner... signers) {
		{{- if ne .ReturnTypeABI "Void"}}
		{{ if eq .ReturnTypeABI  "InteropInterface" }}List<StackItem>{{ else }}NeoInvokeFunction{{ end }} response = null;
		{{- end}}
		try {
			{{ $length := len .Arguments -}}
			{{- if ne .ReturnTypeABI "Void"}}response = {{ end }}{{ if eq .ReturnTypeABI  "InteropInterface" }}smartContract.callFunctionAndUnwrapIterator{{ else }}smartContract.callInvokeFunction{{ end }}(
				"{{ .NameABI }}",
				{{ if and (eq $length 0) (eq .ReturnTypeABI  "InteropInterface") -}}
				Collections.<ContractParameter>emptyList(),
				{{ else if gt $length 0 -}}
				{{- if eq $length 1 -}}
				Collections.singletonList(
				{{- else -}}
				Arrays.asList(
					{{- end -}}
					{{- range $index, $arg := .Arguments }}
					{{ Neow3jWrapParameter $arg.TypeABI }}({{ .Name }}){{ if lt $index (Dec $length) }},{{ end }}
					{{- end }}
				),
				{{ end -}}
				{{- if eq .ReturnTypeABI  "InteropInterface" -}}
				20,
				{{ end -}}
				signers
			);
		} catch (IOException e) {
			throw new RuntimeException(e);
		}
		{{-  if ne .ReturnTypeABI "Void" }}
		return {{ Neow3jReturnTestInvoke .ReturnTypeABI }};
		{{-  end }}
	}
{{- end -}}
package <REPLACE ME>;

import io.neow3j.contract.SmartContract;
import io.neow3j.crypto.ECKeyPair;
import io.neow3j.protocol.Neow3j;
import io.neow3j.protocol.Neow3jConfig;
import io.neow3j.protocol.core.response.NeoInvokeFunction;
import io.neow3j.protocol.core.stackitem.StackItem;
import io.neow3j.protocol.http.HttpService;
import io.neow3j.transaction.AccountSigner;
import io.neow3j.transaction.TransactionBuilder;
import io.neow3j.types.ContractParameter;
import io.neow3j.types.Hash160;
import io.neow3j.types.Hash256;
import io.neow3j.utils.ArrayUtils;

import java.io.IOException;
import java.math.BigInteger;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Map;

public class {{ .ContractName }} {
	Neow3j neow3j;
	Hash160 scriptHash;
	SmartContract smartContract;

	private void setScriptHash(Hash160 scriptHash) {
        this.scriptHash = scriptHash;
    }

    private void setSmartContract(SmartContract smartContract) {
        this.smartContract = smartContract;
    }

    public {{ .ContractName }}(String rpcAddress, Neow3jConfig neow3jConfig) {
        neow3j = Neow3j.build(new HttpService(rpcAddress), neow3jConfig);
        setScriptHash(new Hash160("{{ .Hash }}"));
        setSmartContract(new SmartContract(scriptHash, neow3j));
    }
{{  range $m := .Methods}}
{{- if .Safe }}
{{- template "TESTINVOKEMETHOD" $m -}}
{{- else }}
{{- template "INVOKEMETHOD" $m }}
{{ template "TESTINVOKEMETHOD" $m -}}
{{- end }}
{{end}}
}
`

func generateOffchainSDK(cfg *generators.GenerateCfg) error {
	err := createJavaPackage(cfg)
	defer cfg.ContractOutput.Close()
	if err != nil {
		return err
	}

	cfg.MethodNameConverter = strcase.ToLowerCamel
	cfg.ParamTypeConverter = offchainScParameterTypeToJava
	cfg.SupportMethodOverload = true
	ctr, err := generators.TemplateFromManifest(cfg)
	if err != nil {
		return fmt.Errorf("failed to parse manifest into contract template: %v", err)
	}

	funcMap := template.FuncMap{
		"Neow3jWrapParameter":    neow3jWrapParameterTypes,
		"Neow3jReturnType":       changeListMapReturnTypeJava,
		"Neow3jReturnTestInvoke": offchainJavaReturn,
		"UpperFirst":             generators.UpperFirst,
		"Dec":                    decreaseNumber,
	}

	tmp, err := template.New("generate").Funcs(funcMap).Parse(javaOffChainSrcTmpl)
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

func createOffchainJavaPackage(cfg *generators.GenerateCfg) error {
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

func offchainScParameterTypeToJava(typ smartcontract.ParamType) string {
	switch typ {
	case smartcontract.AnyType:
		return "Object"
	case smartcontract.InteropInterfaceType:
		return "List<?>"
	case smartcontract.BoolType:
		return "boolean"
	case smartcontract.IntegerType:
		return "BigInteger"
	case smartcontract.ByteArrayType:
		return "byte[]"
	case smartcontract.StringType:
		return "String"
	case smartcontract.Hash160Type:
		return "Hash160"
	case smartcontract.Hash256Type:
		return "Hash256"
	case smartcontract.PublicKeyType:
		return "ECKeyPair.ECPublicKey"
	case smartcontract.ArrayType:
		return "List<?>"
	case smartcontract.MapType:
		return "Map<?, ?>"
	case smartcontract.VoidType:
		return "void"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}

func changeListMapReturnTypeJava(typ string) string {
	if typ == "List<?>" {
		return "List<StackItem>"
	} else if typ == "Map<?, ?>" {
		return "Map<StackItem, StackItem>"
	} else {
		return typ
	}
}

func offchainJavaReturn(typ string) string {
	switch typ {
	case "Any":
		return "response.getInvocationResult().getFirstStackItem().getValue()"
	case "InteropInterface":
		return "response"
	case "Boolean":
		return "response.getInvocationResult().getFirstStackItem().getBoolean()"
	case "Integer":
		return "response.getInvocationResult().getFirstStackItem().getInteger()"
	case "ByteArray":
		return "response.getInvocationResult().getFirstStackItem().getByteArray()"
	case "String":
		return "response.getInvocationResult().getFirstStackItem().getString()"
	case "Hash160":
		return "Hash160.fromAddress(response.getInvocationResult().getFirstStackItem().getAddress())"
	case "Hash256":
		return "new Hash256(ArrayUtils.reverseArray(response.getInvocationResult().getFirstStackItem().getByteArray()))"
	case "PublicKey":
		return "new ECKeyPair.ECPublicKey(response.getInvocationResult().getFirstStackItem().getHexString())"
	case "Array":
		return "response.getInvocationResult().getFirstStackItem().getList()"
	case "Map":
		return "response.getInvocationResult().getFirstStackItem().getMap()"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}

func neow3jWrapParameterTypes(typ string) string {
	switch typ {
	case "Any":
		return "ContractParameter.mapToContractParameter"
	case "InteropInterface":
		return "ContractParameter.any"
	case "Boolean":
		return "ContractParameter.bool"
	case "Integer":
		return "ContractParameter.integer"
	case "ByteArray":
		return "ContractParameter.byteArray"
	case "String":
		return "ContractParameter.string"
	case "Hash160":
		return "ContractParameter.hash160"
	case "Hash256":
		return "ContractParameter.hash256"
	case "PublicKey":
		return "ContractParameter.publicKey"
	case "Array":
		return "ContractParameter.array"
	case "Map":
		return "ContractParameter.map"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}

func decreaseNumber(num int) int {
	return num - 1
}
