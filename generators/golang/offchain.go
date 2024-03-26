package golang

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/rpcbinding"
)

func goOffChainConfig() binding.Config {
	return rpcbinding.NewConfig()
}

func goOffChainGenerate() generateFunction {
	return rpcbinding.Generate
}
