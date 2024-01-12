package java

import "cpm/generators"

func GenerateSDK(cfg *generators.GenerateCfg, sdkType string) error {
	if sdkType == generators.SDKOnChain {
		return generateOnchainSDK(cfg)
	} else {
		return generateOffchainSDK(cfg)
	}
}
