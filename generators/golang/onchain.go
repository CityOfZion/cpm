package golang

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
)

func goOnChainConfig() binding.Config {
	return binding.NewConfig()
}

func goOnChainGenerate() generateFunction {
	return binding.Generate
}
