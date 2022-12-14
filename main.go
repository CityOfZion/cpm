package main

import (
	"context"
	"cpm/generators"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"strings"
)

var (
	TOOL_NEO_GO      = "neo-go"
	TOOL_NEO_EXPRESS = "neo-express"

	LANG_GO     = "go"
	LANG_PYTHON = "python"
	LANG_JAVA   = "java"
	LANG_CSHARP = "csharp"

	LOG_INFO  = "INFO"
	LOG_DEBUG = "DEBUG"

	DEFAULT_CONFIG_FILE = "cpm.json"
)

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
				Usage:  "Create a new cpm.json config file",
				Action: handleCliInit,
			},
			{
				Name:  "run",
				Usage: "Download all contracts from cpm.json and generate SDKs where specified",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "download-only", Usage: "Override config settings to only download contracts and storage", Required: false},
					&cli.BoolFlag{Name: "sdk-only", Usage: "Override config settings to only generate SDKs", Required: false},
					//&cli.GenericFlag{
					//	Name:     "d",
					//	Usage:    "Destination toolchain",
					//	Required: false,
					//	Value: &EnumValue{
					//		Enum: []string{TOOL_NEO_EXPRESS},
					//	},
					//},
				},
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
							&cli.StringFlag{Name: "n", Usage: "Source network label. Searches cpm.json for the network by label to find the host", Required: false},
							&cli.StringFlag{Name: "N", Usage: "Source network host", Required: false},
							&cli.StringFlag{Name: "i", Usage: "neo express config file", Required: false, DefaultText: "default.neo-express"},
						},
						Action: handleCliDownloadContract,
					},
					{
						Name:  "manifest",
						Usage: "Download the contract manifest",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "c", Usage: "Contract script hash", Required: true},
							&cli.StringFlag{Name: "n", Usage: "Source network label. Searches cpm.json for the network by label to find the host", Required: false},
							&cli.StringFlag{Name: "N", Usage: "Source network host", Required: false},
						},
						Action: handleCliDownloadManifest,
					},
				},
			},
			{
				Name:   "generate",
				Usage:  "Generate SDK from manifest",
				Action: handleCliGenerate,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "m", Usage: "Path to contract manifest.json", Required: true},
					&cli.StringFlag{Name: "c", Usage: "Contract script hash if known", Required: false},
					&cli.GenericFlag{
						Name:     "l",
						Usage:    "SDK output language",
						Required: true, // TODO: figure out why this is not working
						Value: &EnumValue{
							Enum: []string{LANG_GO, LANG_PYTHON, LANG_JAVA, LANG_CSHARP},
						},
					},
				},
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
	sdkOnly := cCtx.Bool("sdk-only")
	downloadOnly := cCtx.Bool("download-only")
	downloadContracts := !sdkOnly
	generateSDKs := !downloadOnly

	if sdkOnly && downloadOnly {
		log.Fatal("sdk-only and download-only flags are mutually exclusive.")
	}

	LoadConfig()

	var downloader Downloader
	// for now we only support NeoExpress
	downloader = NewNeoExpressDownloader(cfg.Tools.NeoExpress.ConfigPath)

	for _, c := range cfg.Contracts {
		log.Infof("Processing contract '%s' (%s)", c.Label, c.ScriptHash.StringLE())

		hosts := cfg.getHosts(*c.SourceNetwork)

		success := false
		generateSuccess := false
		skipGenerate := *c.ContractGenerateSdk == false
		for _, host := range hosts {
			if downloadContracts {
				log.Debugf("Attempting to download contract '%s' (%s) using NEOXP from network %s", c.Label, c.ScriptHash.StringLE(), host)
				message, err := downloader.downloadContract(c.ScriptHash, host)
				if err != nil {
					// just log the error we got from the downloader and try the next host
					log.Debug(message)
				} else {
					log.Info(message)
					success = true

					if generateSDKs && !skipGenerate {
						err := fetchManifestAndGenerateSDK(&c, host)
						if err != nil {
							log.Debug(err)
						} else {
							generateSuccess = true
						}
					}
					break
				}
			}
			if sdkOnly && *c.ContractGenerateSdk {
				err := fetchManifestAndGenerateSDK(&c, host)
				if err != nil {
					log.Fatal(err)
				}
				generateSuccess = true
				break
			}
		}

		if downloadContracts && !success {
			log.Fatalf("Failed to download contract '%s' (%s). Use '--log-level DEBUG' for more information", c.Label, c.ScriptHash)
		}

		if generateSDKs && !skipGenerate && !generateSuccess {
			log.Fatalf("Failed to generate SDK for contract '%s' (%s). Use '--log-level DEBUG' for more information", c.Label, c.ScriptHash)
		}
	}

	return nil
}

func handleCliDownloadContract(cCtx *cli.Context) error {
	networkLabel := cCtx.String("n")
	networkHost := cCtx.String("N")

	var (
		scriptHash util.Uint160
		downloader Downloader
	)

	if networkLabel != "" && networkHost != "" {
		log.Fatal("-n and -N flags are mutually exclusive")
	}

	scriptHash, err := util.Uint160DecodeStringLE(strings.TrimPrefix(cCtx.String("c"), "0x"))
	if err != nil {
		return err
	}

	LoadConfig()

	var hosts []string
	if len(networkLabel) > 0 {
		hosts = cfg.getHosts(networkLabel)
	} else if len(networkHost) > 0 {
		// TODO: sanity check value
		hosts = []string{networkHost}
	} else {
		log.Fatal("Must specify either -n or -N flag")
	}

	// for now, we only support NeoExpress
	configPath := cfg.Tools.NeoExpress.ConfigPath
	tmp := cCtx.String("i")
	if len(tmp) > 0 {
		configPath = tmp
	}
	downloader = NewNeoExpressDownloader(configPath)

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
		log.Fatalf("Failed to download contract %s. Use '--log-level DEBUG' for more information", scriptHash)
	}
	return nil
}

func handleCliDownloadManifest(cCtx *cli.Context) error {
	networkLabel := cCtx.String("n")
	networkHost := cCtx.String("N")

	if networkLabel != "" && networkHost != "" {
		log.Fatal("-n and -N flags are mutually exclusive")
	}

	var hosts []string
	if len(networkLabel) > 0 {
		LoadConfig()
		hosts = cfg.getHosts(networkLabel)
	} else if len(networkHost) > 0 {
		// TODO: sanity check value
		hosts = []string{networkHost}
	} else {
		log.Fatal("Must specify either -n or -N flag")
	}

	scriptHash, err := util.Uint160DecodeStringLE(strings.TrimPrefix(cCtx.String("c"), "0x"))
	if err != nil {
		return err
	}

	for _, host := range hosts {
		m, err := fetchManifest(&scriptHash, host)
		if err != nil {
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
			return nil
		}
	}

	log.Fatalf("Failed to fetch manifest. Use '--log-level DEBUG' for more information")
	return err
}

func handleCliGenerate(cCtx *cli.Context) error {
	m, _, err := readManifest(cCtx.String("m"))
	if err != nil {
		log.Fatalf("can't read contract manifest: %s", err)
	}

	contractHash, _ := util.Uint160DecodeStringLE(strings.TrimPrefix(cCtx.String("c"), "0x"))

	language := cCtx.String("l")
	return generateSDK(m, contractHash, language)
}

func fetchManifestAndGenerateSDK(c *ContractConfig, host string) error {
	m, err := fetchManifest(&c.ScriptHash, host)
	if err != nil {
		return err
	}

	err = generateSDK(m, c.ScriptHash, cfg.Defaults.SdkLanguage)
	if err != nil {
		return err
	}
	return nil
}

func fetchManifest(scriptHash *util.Uint160, host string) (*manifest.Manifest, error) {
	opts := rpcclient.Options{}
	client, err := rpcclient.New(context.TODO(), host, opts)
	err = client.Init()
	if err != nil {
		log.Debug("RPCClient init failed with %v", err)
		return nil, err
	}
	state, err := client.GetContractStateByHash(*scriptHash)
	if err != nil {
		log.Debug("get contractstate failed with %v", err)
		return nil, err
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

func generateSDK(m *manifest.Manifest, scriptHash util.Uint160, language string) error {
	cfg := generators.GenerateCfg{
		Manifest:     m,
		ContractHash: scriptHash,
	}

	var err error
	if language == LANG_PYTHON {
		err = generators.GeneratePythonSDK(&cfg)
	} else if language == LANG_JAVA {
		err = generators.GenerateJavaSDK(&cfg)
	} else if language == LANG_CSHARP {
		err = generators.GenerateCsharpSDK(&cfg)
	} else if language == LANG_GO {
		err = generators.GenerateGoSDK(&cfg)
	} else {
		log.Fatalf("%s is unsupported", language)
	}

	if err != nil {
		return err
	}
	return nil
}
