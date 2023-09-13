package generators

import (
	"fmt"
	"os"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	log "github.com/sirupsen/logrus"
)

/*
	Creates a TypeScript SDK that can be easily used when trying to invoke a smart contract.
	Given a contract named `Sample Contract`, the output is a folder with the following structure:
		.
		├── SampleContract
		│   ├── api.ts
		│   ├── index.ts
		│   └── SampleContract.ts

		which can be used in your TypeScript project with

			import { SampleContract } from './SampleContract'
			const sampleContract = new SampleContract({
				SampleContract.SCRIPT_HASH,
				invoker: await NeonInvoker.init({ rpcAddress: 'https://mainnet1.neo.coz.io:443' }),
				parser: NeonParser,
				eventListener: new NeonEventListener('https://mainnet1.neo.coz.io:443')
			})

			const txId = sampleContract.func1()
			const testInvokeResponse = sampleContract.testFunc1()
*/

const typescriptSrcApiTmpl = `
{{- define "APIMETHOD" }}
export function {{ .Name }}API(scriptHash: string{{if .Arguments}}, params: { {{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}{{- .Name}}: {{.Type}}
{{- end}} }, parser: Neo3Parser {{end}}): ContractInvocation {
	return {
		scriptHash,
		operation: '{{ .NameABI }}',
		args: [{{range $index, $arg := .Arguments -}}
			parser.formatRpcArgument(params.{{- .Name}}, { type: '{{ .TypeABI }}' }),
		{{- end}}
		],
	}
}
{{- end -}}
import { Neo3Parser, ContractInvocation} from "@cityofzion/neon-dappkit-types"

{{- range $m := .Methods}}
{{ template "APIMETHOD" $m -}}
{{end}}
`

const typescriptSrcClassTmpl = `
{{- define "INVOKEMETHOD" }}
	async {{ .Name }}({{if .Arguments}}params: { {{range $index, $arg := .Arguments -}}
			{{- if ne $index 0}}, {{end}}{{- .Name}}: {{.Type}}
		{{- end}} } {{end}}){{if .ReturnType }}: Promise<string>{{ else }} {{end}}{
		return await this.config.invoker.invokeFunction({
			invocations: [Invocation.{{ .Name }}API(this.config.scriptHash{{if .Arguments}}, params, this.config.parser{{end}})],
			signers: [],
		})
	}
{{- end -}}
{{- define "ITERATORGENERATORMETHOD" }}
	async* {{if not .Safe}}test{{ upperFirst .Name }}{{else}}{{ .Name }}{{end}}({{if .Arguments}}params: { {{range $index, $arg := .Arguments -}}
		{{- if ne $index 0}}, {{end}}{{- .Name}}: {{.Type}}
	{{- end}} } {{end}}): AsyncGenerator {
		const res = await this.config.invoker.testInvoke({
			invocations: [Invocation.{{ .Name }}API(this.config.scriptHash{{if .Arguments}}, params, this.config.parser{{end}})],
			signers: [],
		})

		if (res.stack.length !== 0 && res.session !== undefined && typeChecker.isStackTypeInteropInterface(res.stack[0])) {

			let iterator = await this.config.invoker.traverseIterator(res.session, res.stack[0].id, 1)

			while (iterator.length !== 0){
				if (typeChecker.isStackTypeInteropInterface(iterator[0])){
					throw new Error(res.exception ?? 'can not have an iterator inside another iterator')
				}else{
					yield this.config.parser.parseRpcResponse(iterator[0])
					iterator = await this.config.invoker.traverseIterator(res.session, res.stack[0].id, 1)
				}
			}
		}
		else {
			throw new Error(res.exception ?? 'unrecognized response')
		}
	}
{{- end -}}
{{- define "TESTINVOKEMETHOD" }}
	async {{if not .Safe}}test{{ upperFirst .Name }}{{else}}{{ .Name }}{{end}}({{if .Arguments}}params: { {{range $index, $arg := .Arguments -}}
		{{- if ne $index 0}}, {{end}}{{- .Name}}: {{.Type}}
	{{- end}} } {{end}}){{if .ReturnType }}: Promise<{{ .ReturnType }}>{{ else }} {{end}}{
		const res = await this.config.invoker.testInvoke({
			invocations: [Invocation.{{ .Name }}API(this.config.scriptHash{{if .Arguments}}, params, this.config.parser{{end}})],
			signers: [],
		})

		if (res.stack.length === 0) {
			throw new Error(res.exception ?? 'unrecognized response')
		}
		{{- if ne .ReturnType "void"}}
		
		return this.config.parser.parseRpcResponse(res.stack[0], { type: '{{ .ReturnTypeABI }}' })
		{{- end}}
	}
{{- end -}}
{{- define "TESTMETHOD" }}
	{{- if eq .ReturnTypeABI "InteropInterface" }}
	{{- template "ITERATORGENERATORMETHOD" . -}}
	{{ else }}
	{{- template "TESTINVOKEMETHOD" . -}}
	{{- end -}}
{{- end -}}
{{- define "EVENTLISTENER" }}
	async confirm{{ upperFirst .Name }}Event(txId: string): Promise<void>{
		if (!this.config.eventListener) throw new Error('EventListener not provided')

		const txResult = await this.config.eventListener.waitForApplicationLog(txId)
		this.config.eventListener.confirmTransaction(
			txResult, {contract: this.config.scriptHash, eventname: '{{ .Name }}'}
		)
	}

	listen{{ upperFirst .Name }}Event(callback: Neo3EventListenerCallback): void{
		if (!this.config.eventListener) throw new Error('EventListener not provided')
		
		this.config.eventListener.addEventListener(this.config.scriptHash, '{{ .Name }}', callback)
	}

	remove{{ upperFirst .Name }}EventListener(callback: Neo3EventListenerCallback): void{
		if (!this.config.eventListener) throw new Error('EventListener not provided')
		
		this.config.eventListener.removeEventListener(this.config.scriptHash, '{{ .Name }}', callback)
	}
{{- end -}}
import { Neo3EventListener, Neo3EventListenerCallback, Neo3Invoker, Neo3Parser } from "@cityofzion/neon-dappkit-types"
import { typeChecker } from "@cityofzion/neon-dappkit"
import * as Invocation from './api'

export type SmartContractConfig = {
  scriptHash: string;
  invoker: Neo3Invoker;
  parser?: Neo3Parser;
  eventListener?: Neo3EventListener | null;
}

export class {{ .ContractName }}{
  static SCRIPT_HASH = '{{ .Hash }}'

  private config: Required<SmartContractConfig>

	constructor(configOptions: SmartContractConfig) {
		this.config = { 
			...configOptions, 
			parser: configOptions.parser ?? require("@cityofzion/neon-dappkit").NeonParser,
			eventListener: configOptions.eventListener ?? null
		}
	}

{{- range $e := .Events}}
{{ template "EVENTLISTENER" $e -}}
{{end}}
{{- range $m := .Methods}}
{{if .Safe -}}
{{ template "TESTMETHOD" $m -}}
{{- else -}}
{{ template "INVOKEMETHOD" $m }}
{{ template "TESTMETHOD" $m -}}
{{end -}}
{{end}}
}
`

const typescriptSrcIndexTmpl = `export * from './{{ .ContractName }}'
export * from './api'`

func GenerateTypeScriptSDK(cfg *GenerateCfg) error {
	cfg.MethodNameConverter = strcase.ToLowerCamel
	cfg.ParamTypeConverter = scTypeToTypeScript
	ctr, err := templateFromManifest(cfg)
	if err != nil {
		return fmt.Errorf("failed to parse manifest into contract template: %v", err)
	}

	sdkDir := cfg.SdkDestination + ctr.ContractName
	err = os.MkdirAll(sdkDir, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory %s: %w", sdkDir, err)
	}

	err = generateTypeScriptSdkFile(cfg, ctr, sdkDir, "api", typescriptSrcApiTmpl)
	if err != nil {
		return err
	}

	err = generateTypeScriptSdkFile(cfg, ctr, sdkDir, ctr.ContractName, typescriptSrcClassTmpl)
	if err != nil {
		return err
	}

	err = generateTypeScriptSdkFile(cfg, ctr, sdkDir, "index", typescriptSrcIndexTmpl)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	sdkLocation := wd + "/" + cfg.SdkDestination + upperFirst(cfg.Manifest.Name)
	log.Infof("Created SDK for contract '%s' at %s with contract hash 0x%s", cfg.Manifest.Name, sdkLocation, cfg.ContractHash.StringLE())

	return nil
}

func generateTypeScriptSdkFile(cfg *GenerateCfg, ctr contractTmpl, sdkDir string, fileName string, templateString string) error {
	err := createTypeScriptSdkFile(cfg, sdkDir, fileName)
	defer cfg.ContractOutput.Close()
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"upperFirst": upperFirst,
	}

	tmp, err := template.New("generate").Funcs(funcMap).Parse(templateString)
	if err != nil {
		return fmt.Errorf("failed to parse TypeScript source %s file template: %v", fileName, err)
	}

	err = tmp.Execute(cfg.ContractOutput, ctr)
	if err != nil {
		return fmt.Errorf("failed to generate TypeScript %s file code using template: %v", fileName, err)
	}

	return nil
}

func createTypeScriptSdkFile(cfg *GenerateCfg, sdkDir string, fileName string) error {
	f, err := os.Create(sdkDir + "/" + fileName + ".ts")
	if err != nil {
		f.Close()
		return fmt.Errorf("can't create %s.ts file: %w", fileName, err)
	} else {
		cfg.ContractOutput = f
	}
	return nil
}

func scTypeToTypeScript(typ smartcontract.ParamType) string {
	switch typ {
	case smartcontract.AnyType:
		return "any"
	case smartcontract.BoolType:
		return "boolean"
	case smartcontract.InteropInterfaceType:
		return "object"
	case smartcontract.IntegerType:
		return "number"
	case smartcontract.ByteArrayType:
		return "string"
	case smartcontract.StringType:
		return "string"
	case smartcontract.Hash160Type:
		return "string"
	case smartcontract.Hash256Type:
		return "string"
	case smartcontract.PublicKeyType:
		return "string"
	case smartcontract.ArrayType:
		return "any[]"
	case smartcontract.MapType:
		return "object"
	case smartcontract.VoidType:
		return "void"
	default:
		panic(fmt.Sprintf("unknown type: %T %s", typ, typ))
	}
}
