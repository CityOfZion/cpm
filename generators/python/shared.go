package python

import "cpm/generators"

func GenerateSDK(cfg *generators.GenerateCfg, sdkType string) error {
	if sdkType == "onchain" {
		return generateOnchainSDK(cfg)
	} else {
		return generateOffchainSDK(cfg)
	}
}
