package python

import "cpm/generators"

func GenerateSDK(cfg *generators.GenerateCfg, sdkType string) error {
	if sdkType == generators.SDK_ONCHAIN {
		return generateOnchainSDK(cfg)
	} else {
		return generateOffchainSDK(cfg)
	}
}
