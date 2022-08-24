# Installation
Download the tool from the releases page and place it somewhere on your path

# Usage

```shell
cpm -h
```

`cpm.json` is your project configuration file. Have a look or read more about it [here](docs/config.md).

## Example commands

### Download all contracts listed in `cpm.json`
Note that only `neo-express` is supported as destination chain. An [issue](https://github.com/nspcc-dev/neo-go/issues/2406) for `neo-go` to add support (go vote!).

```shell
cpm --log-level DEBUG run 
```

### Download a single contract or contract manifest
```shell
cpm download contract -c 0x4380f2c1de98bb267d3ea821897ec571a04fe3e0 -n mainnet
cpm download manifest -c 0x4380f2c1de98bb267d3ea821897ec571a04fe3e0 -N https://mainnet1.neo.coz.io:443
```

### Build SDK from local manifest
```shell
cpm generate -m samplecontract.manifest.json -l python
cpm generate -m samplecontract.manifest.json -l go
```
Note: the name from the manifest is used as output name
