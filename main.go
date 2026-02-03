package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cpm/generators"
	"cpm/generators/csharp"
	"cpm/generators/golang"
	"cpm/generators/java"
	"cpm/generators/python"
	"cpm/generators/typescript"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	LANG_GO         = "go"
	LANG_PYTHON     = "python"
	LANG_JAVA       = "java"
	LANG_CSHARP     = "csharp"
	LANG_TYPESCRIPT = "ts"

	LOG_INFO  = "INFO"
	LOG_DEBUG = "DEBUG"

	DEFAULT_CONFIG_FILE = "cpm.yaml"

	version = "dev"
)

var GenerateCommandHelpTemplate = `NAME:
   {{template "helpNameTemplate" .}}

USAGE:
   {{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.HelpName}} {{if .VisibleFlags}}language [language options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Description}}

DESCRIPTION:
   {{template "descriptionTemplate" .}}{{end}}{{if .VisibleCommands}}

LANGUAGES:{{template "visibleCommandTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

OPTIONS:{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

OPTIONS:{{template "visibleFlagTemplate" .}}{{end}}
`

func main() {
	log.SetOutput(os.Stdout)

	app := &cli.App{
		Usage: "Contract Package Manager",
		Flags: []cli.Flag{
			&cli.GenericFlag{
				Name:     "log-level",
				Usage:    "Log output level",
				Required: false,
				Value: &EnumValue{
					Enum: []string{LOG_INFO, LOG_DEBUG},
				},
			},
		},
		Before: beforeAction,
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() == 0 {
				cli.ShowAppHelpAndExit(cCtx, 0)
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:   "init",
				Usage:  "Create a new cpm.yaml config file",
				Action: handleCliInit,
			},
			{
				Name:   "run",
				Usage:  "Download all contracts from cpm.yaml and generate SDKs where specified",
				Action: handleCliRun,
			},
			{
				Name:  "download",
				Usage: "Download contract or manifest",
				Subcommands: []*cli.Command{
					{
						Name:  "contract",
						Usage: "Download a single contract",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "c", Usage: "Contract script hash", Required: true},
							&cli.StringFlag{Name: "n", Usage: "Source network label. Searches cpm.yaml for the network by label to find the host", Required: false},
							&cli.StringFlag{Name: "N", Usage: "Source network host", Required: false},
							&cli.StringFlag{Name: "i", Usage: "Neo express config file", Required: false, DefaultText: "default.neo-express"},
							&cli.BoolFlag{Name: "s", Usage: "Save contract to the 'contracts' section of cpm.yaml", Required: false, Value: false, DisableDefaultText: true},
						},
						Action: handleCliDownloadContract,
					},
					{
						Name:  "manifest",
						Usage: "Download the contract manifest",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "c", Usage: "Contract script hash", Required: true},
							&cli.StringFlag{Name: "n", Usage: "Source network label. Searches cpm.yaml for the network by label to find the host", Required: false},
							&cli.StringFlag{Name: "N", Usage: "Source network host", Required: false},
							&cli.BoolFlag{Name: "s", Usage: "Save contract to the 'contracts' section of cpm.yaml", Required: false, Value: false, DisableDefaultText: true},
						},
						Before: func(c *cli.Context) error {
							networkLabel := c.String("n")
							networkHost := c.String("N")
							if networkLabel == "" && networkHost == "" {
								return fmt.Errorf("must to specify either a network label using '-n' or a network host using '-N'")
							}
							return nil
						},
						Action: handleCliDownloadManifest,
					},
				},
			},
			{
				Name:               "generate",
				Usage:              "Generate SDK from manifest",
				CustomHelpTemplate: GenerateCommandHelpTemplate,
				Subcommands: []*cli.Command{
					{
						Name:  LANG_GO,
						Usage: "Generate a SDK for use with Golang",
						Action: func(c *cli.Context) error {
							return handleCliGenerate(c, LANG_GO)
						},
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "m", Usage: "Path to contract manifest.json", Required: true},
							&cli.StringFlag{Name: "c", Usage: "Contract script hash if known", Required: false},
							&cli.StringFlag{Name: "o", Usage: "Output folder", Required: false},
							&cli.GenericFlag{
								Name:     "t",
								Usage:    "SDK type",
								Required: true,
								Value: &EnumValue{
									Enum: []string{generators.SDKOffChain, generators.SDKOnChain},
								},
							},
						},
					},
					{
						Name:  LANG_PYTHON,
						Usage: "Generate a SDK for use with Python",
						Action: func(c *cli.Context) error {
							return handleCliGenerate(c, LANG_PYTHON)
						},
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "m", Usage: "Path to contract manifest.json", Required: true},
							&cli.StringFlag{Name: "c", Usage: "Contract script hash if known", Required: false},
							&cli.StringFlag{Name: "o", Usage: "Output folder", Required: false},
							&cli.GenericFlag{
								Name:     "t",
								Usage:    "SDK type",
								Required: true,
								Value: &EnumValue{
									Enum: []string{generators.SDKOffChain, generators.SDKOnChain},
								},
							},
						},
					},
					{
						Name:  LANG_JAVA,
						Usage: "Generate a SDK for use with Java",
						Action: func(c *cli.Context) error {
							return handleCliGenerate(c, LANG_JAVA)
						},
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "m", Usage: "Path to contract manifest.json", Required: true},
							&cli.StringFlag{Name: "c", Usage: "Contract script hash if known", Required: false},
							&cli.StringFlag{Name: "o", Usage: "Output folder", Required: false},
							&cli.GenericFlag{
								Name:     "t",
								Usage:    "SDK type",
								Required: true,
								Value: &EnumValue{
									Enum: []string{generators.SDKOffChain, generators.SDKOnChain},
								},
							},
						},
					},
					{
						Name:  LANG_CSHARP,
						Usage: "Generate an on-chain SDK for use with C#",
						Action: func(c *cli.Context) error {
							return handleCliGenerate(c, LANG_CSHARP)
						},
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "m", Usage: "Path to contract manifest.json", Required: true},
							&cli.StringFlag{Name: "c", Usage: "Contract script hash if known", Required: false},
							&cli.StringFlag{Name: "o", Usage: "Output folder", Required: false},
						},
					},
					{
						Name:  LANG_TYPESCRIPT,
						Usage: "Generate an off-chain SDK for use with TypeScript",
						Action: func(c *cli.Context) error {
							return handleCliGenerate(c, LANG_TYPESCRIPT)
						},
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "m", Usage: "Path to contract manifest.json", Required: true},
							&cli.StringFlag{Name: "c", Usage: "Contract script hash if known", Required: false},
							&cli.StringFlag{Name: "o", Usage: "Output folder", Required: false},
						},
					},
				},
			},
			{
				Name:   "version",
				Usage:  "Shows CPM version",
				Action: handleCliVersion,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func beforeAction(cCtx *cli.Context) error {
	if cCtx.String("log-level") == LOG_DEBUG {
		log.SetLevel(log.DebugLevel)
	}
	return nil
}

func handleCliInit(*cli.Context) error {
	CreateDefaultConfig()
	return nil
}

func handleCliRun(cCtx *cli.Context) error {
	LoadConfig()

	var downloader Downloader
	// for now we only support NeoExpress
	downloader = NewNeoExpressDownloader(cfg.Tools.NeoExpress.ConfigPath)

	for _, c := range cfg.Contracts {
		log.Infof("Processing contract '%s' (%s)", c.Label, c.ScriptHash.StringLE())
		hosts := cfg.getHosts(*c.SourceNetwork)

		if *c.Download {
			downloadSuccess := false
			for _, host := range hosts {
				log.Debugf("Attempting to download contract '%s' (%s) using NEOXP from network %s", c.Label, c.ScriptHash.StringLE(), host)
				message, err := downloader.downloadContract(c.ScriptHash, host)
				if err == nil {
					log.Info(message)
					downloadSuccess = true
					break
				}
				// just log the error we got from the downloader and try the next host
				log.Debug(message)
			}

			if !downloadSuccess {
				log.Fatalf("Failed to download contract '%s' (%s). Use '--log-level DEBUG' for more information", c.Label, c.ScriptHash.StringLE())
			}
		} else {
			log.Debugf("Skipping contract download")
		}

		if *c.GenerateSdk {
			generateSuccess := false
			for _, host := range hosts {
				err := fetchManifestAndGenerateSDK(&c, host)
				if err == nil {
					generateSuccess = true
					break
				}
				log.Debug(err)
			}

			if !generateSuccess {
				log.Fatalf("Failed to generate SDK for contract '%s' (%s). Use '--log-level DEBUG' for more information", c.Label, c.ScriptHash.StringLE())
			}
		} else {
			log.Debugf("Skipping SDK generation")
		}
	}

	return nil
}

func handleCliDownloadContract(cCtx *cli.Context) error {
	networkLabel := cCtx.String("n")
	networkHost := cCtx.String("N")
	contractHash := cCtx.String("c")
	configPath := cCtx.String("i")
	saveContract := cCtx.Bool("s")

	LoadConfig()
	hosts, err := getHosts(networkLabel, networkHost)
	if err != nil {
		return err
	}

	// for now, we only support NeoExpress
	downloader := NewNeoExpressDownloader(configPath)
	return downloadContract(hosts, contractHash, downloader, saveContract, false)
}

func handleCliDownloadManifest(cCtx *cli.Context) error {
	networkLabel := cCtx.String("n")
	networkHost := cCtx.String("N")
	contractHash := cCtx.String("c")
	saveContract := cCtx.Bool("s")

	var hosts []string
	var err error
	if networkHost != "" && !saveContract {
		hosts = []string{networkHost}
	} else {
		LoadConfig()
		hosts, err = getHosts(networkLabel, networkHost)
		if err != nil {
			return err
		}
	}
	return downloadManifest(hosts, contractHash, saveContract, false)
}

func handleCliGenerate(cCtx *cli.Context, language string) error {
	m, _, err := readManifest(cCtx.String("m"))
	if err != nil {
		log.Fatalf("can't read contract manifest: %s", err)
	}

	sdkType := cCtx.String("t")
	if sdkType == "" {
		if language == LANG_TYPESCRIPT {
			sdkType = generators.SDKOffChain
		} else {
			sdkType = generators.SDKOnChain
		}
	}

	dest := cCtx.String("o")
	if dest == "" {
		dest = cfg.getSdkDestination(language, sdkType)
	} else {
		dest = EnsureSuffix(dest)
	}

	scriptHash := util.Uint160{}
	scriptHashStr := cCtx.String("c")
	if scriptHashStr != "" {
		scriptHash, err = util.Uint160DecodeStringLE(strings.TrimPrefix(cCtx.String("c"), "0x"))
		if err != nil {
			log.Fatalf("failed to convert script hash: %v", err)
		}
	}
	return generateSDK(&generators.GenerateCfg{Manifest: m, ContractHash: scriptHash, SdkDestination: dest}, language, sdkType)
}

func handleCliVersion(cCtx *cli.Context) error {
	fmt.Printf("cpm %s\n", version)
	return nil
}

// 'cfg' is expected to be initialized
func downloadContract(hosts []string, contractHash string, downloader Downloader, saveContract, testing bool) error {
	scriptHash, err := util.Uint160DecodeStringLE(strings.TrimPrefix(contractHash, "0x"))
	if err != nil {
		return fmt.Errorf("failed to convert script hash: %v", err)
	}

	if saveContract {
		cfg.addContract("unknown", scriptHash)
		if !testing {
			cfg.saveToDisk()
		}
	}

	success := false
	for _, host := range hosts {
		message, err := downloader.downloadContract(scriptHash, host)
		if err != nil {
			// just log the error we got from the downloader and try the next host
			log.Debug(message)
		} else {
			log.Info(message)
			success = true
			break
		}
	}

	if !success {
		return fmt.Errorf("failed to download contract %s. Use '--log-level DEBUG' for more information", scriptHash)
	}
	return nil
}

func downloadManifest(hosts []string, contractHash string, saveContract, testing bool) error {
	scriptHash, err := util.Uint160DecodeStringLE(strings.TrimPrefix(contractHash, "0x"))
	if err != nil {
		return fmt.Errorf("failed to convert script hash: %v", err)
	}

	for _, host := range hosts {
		m, err := fetchManifest(&scriptHash, host)
		if err != nil {
			log.Debug(err)
			continue
		} else {
			f, err := os.Create("contract.manifest.json")
			if err != nil {
				return err
			}

			out, err := json.MarshalIndent(m, "", "   ")
			if err != nil {
				return err
			}

			_, err = f.Write(out)
			if err != nil {
				return err
			}
			log.Info("Written manifest to contract.manifest.json")

			if saveContract {
				cfg.addContract(m.Name, scriptHash)
				if !testing {
					cfg.saveToDisk()
				}
			}

			return nil
		}
	}
	return fmt.Errorf("failed to fetch manifest. Use '--log-level DEBUG' for more information")
}

// must fetch and generate an SDK. Must return an error if generation failed or nothing is generated
func fetchManifestAndGenerateSDK(c *ContractConfig, host string) error {
	m, err := fetchManifest(&c.ScriptHash, host)
	if err != nil {
		return err
	}

	var onChainLanguages []string = nil
	if c.OnChain != nil {
		onChainLanguages = c.OnChain.Languages
	} else if cfg.Defaults.OnChain != nil {
		onChainLanguages = cfg.Defaults.OnChain.Languages
	}

	if onChainLanguages != nil {
		for _, l := range onChainLanguages {
			err = generateSDK(&generators.GenerateCfg{Manifest: m, ContractHash: c.ScriptHash, SdkDestination: cfg.getSdkDestination(l, generators.SDKOnChain)}, l, generators.SDKOnChain)
			if err != nil {
				return err
			}
		}
	}

	var offChainLanguages []string = nil
	if c.OffChain != nil {
		offChainLanguages = c.OffChain.Languages
	} else if cfg.Defaults.OffChain != nil {
		offChainLanguages = cfg.Defaults.OffChain.Languages
	}

	if offChainLanguages != nil {
		for _, l := range offChainLanguages {
			err = generateSDK(&generators.GenerateCfg{Manifest: m, ContractHash: c.ScriptHash, SdkDestination: cfg.getSdkDestination(l, generators.SDKOffChain)}, l, generators.SDKOffChain)
			if err != nil {
				return err
			}
		}
	}

	if onChainLanguages == nil && offChainLanguages == nil {
		return fmt.Errorf("nothing to generate. Ensure your 'cpm.yaml' has an 'onchain' or 'offchain' key under " +
			"the 'defaults' section or contract specific section with at least one language specified")
	}

	return nil
}

func fetchManifest(scriptHash *util.Uint160, host string) (*manifest.Manifest, error) {
	opts := rpcclient.Options{}
	client, err := rpcclient.New(context.TODO(), host, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %v", err)
	}

	err = client.Init()
	if err != nil {
		return nil, fmt.Errorf("RPCClient init failed with: %v", err)
	}

	log.Debugf("Attempting to fetch manifest for contract '%s' using %s", scriptHash.StringLE(), host)
	state, err := client.GetContractStateByHash(*scriptHash)
	if err != nil {
		return nil, fmt.Errorf("getcontractstate failed with: %v", err)
	}
	return &state.Manifest, nil
}

func readManifest(filename string) (*manifest.Manifest, []byte, error) {
	if len(filename) == 0 {
		return nil, nil, fmt.Errorf("no manifest file was found, specify manifest file with '-m' flag")
	}

	manifestBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	m := new(manifest.Manifest)
	err = json.Unmarshal(manifestBytes, m)
	if err != nil {
		return nil, nil, err
	}
	return m, manifestBytes, nil
}

func generateSDK(cfg *generators.GenerateCfg, language, sdkType string) error {
	var err error
	if language == LANG_PYTHON {
		err = python.GenerateSDK(cfg, sdkType)
	} else if language == LANG_JAVA {
		err = java.GenerateSDK(cfg, sdkType)
	} else if language == LANG_CSHARP {
		err = csharp.GenerateCsharpSDK(cfg)
	} else if language == LANG_GO {
		err = golang.GenerateSDK(cfg, sdkType)
	} else if language == LANG_TYPESCRIPT {
		err = typescript.GenerateTypeScriptSDK(cfg)
	} else {
		log.Fatalf("language '%s' is unsupported", language)
	}

	if err != nil {
		return err
	}
	return nil
}

func getHosts(networkLabel, networkHost string) ([]string, error) {
	if networkLabel != "" && networkHost != "" {
		return nil, fmt.Errorf("-n and -N flags are mutually exclusive")
	}

	var hosts []string
	if len(networkLabel) > 0 {
		hosts = cfg.getHosts(networkLabel)
	} else if len(networkHost) > 0 {
		// TODO: sanity check value
		hosts = []string{networkHost}
	} else {
		return nil, fmt.Errorf("must specify either -n or -N flag")
	}
	return hosts, nil
}
