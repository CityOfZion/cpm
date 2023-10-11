`cpm.yaml` is your project configuration file. It holds all information about which contracts it should download,
from which network and whether it should generate an SDK. Depending on the SDK type generated it allows it to either be
consumed in the smart contract you're developing (=on-chain), or to be consumed in a backend to interact with the smart
contract you developed once deployed (=off-chain).

It has 4 major sections which will be described in detail later on
* `defaults` - this section holds settings that apply to all contracts unless explicitly overridden in the `contracts` section.
* `contracts` - this section describes which contracts to download or generate SDKs for and with what options.
* `tools` - this section describes the available tools and if they can be used for contract downloading and/or generating SDKs.
* `networks` - this section holds a list of networks with corresponding RPC server addresses to the networks used for source information downloading.

# defaults
* `contract-source-network` - describes which network is the source for downloading contracts from. Valid values are [networks.label](#Networks)s.
* `contract-download` - set to `true` to download all [contracts](#contracts) to your local chain.
* `contract-generate-sdk` - set to `true` to generate SDKs for all [contracts](#contracts).
* `on-chain` - describes settings for generating SDKs for use in on chain contracts. See [GenerateConfig](#GenerateConfig).
* `off-chain` - describes settings for generating off-chain SDKs to interact with on chain contracts. See [GenerateConfig](#GenerateConfig).


## GenerateConfig
* `languages` - a list of target languages to generate the SDK in. 
   * Valid values for `on-chain`: `csharp`, `go`, `java` and `python`.
   * Valid values for `off-chain`: `ts` and `python`.
* `destinations` - override default output path per language. Example
```yaml
  on-chain:
    languages:
      - python
      - go
    destinations:
      python: custom_out_py
      go: custom_out_go
  off-chain:
    languages:
      - ts
    destinations:
      ts: custom_out_ts
```

# contracts
* `label` - a user defined label to identify the target contract in the config. Must be a string. Not used elsewhere.
* `script-hash` - the script hash identifying the contract in `0x<hash>` format. i.e. `0x36d0bf624b90a9dad39d85dcafc83f14dab0272f`.
* `source-network` - (Optional) overrides the `contract-source-network` setting in `defaults` to set the source for downloading the contract from. Valid values are [networks.label](#Networks)s.
* `generate-sdk` - (Optional) overrides the `contract-generate-sdk` setting in `defaults` to generate an SDK. Must be a bool value.
* `download` - (Optional) overrides the `contract-download` setting in `defaults` to download a contract to the local chain. Must be a bool value.

# tools
Currently `neo-express` is the only tool that supports downloading contracts. An [issue](https://github.com/nspcc-dev/neo-go/issues/2406) exists for `neo-go` to add download support.
For on-chain SDK generation `C#`, `Java`, `Golang` and `Python` are supported. For off-chain SDK generation `ts` and `Python` are supported.

Each tool must specify the following 2 keys
* `canGenerateSDK` - indicates if the tool can be used for generating SDKs. Must be a bool value.
* `canDownloadContract` - indicates if the tool can be used for downloading contracts. Must be a bool value.

Other keys are tool specific
* `neo-express`
    * `express-path` - where to find the `neoxp` executable. Set to `null` if installed globally. Otherwise, specify the full path including the program name.
    * `config-path` - where to find the `*.neo-express` configuration file of the target network. Must include the file name. i.e. `default.neo-express` if the file is in the root directory.

Example

```yaml
neo-express:
  canGenerateSDK: false
  canDownloadContract: true
  executable-path: null
  config-path: default.neo-express
```

# networks
* `label` - a user defined name for your network. Must be a string.
* `hosts` - a list of RPC addresses that all point to the same network. They will be queried in order until one of them gives a successful response.