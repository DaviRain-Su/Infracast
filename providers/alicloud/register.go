package alicloud

import "github.com/DaviRain-Su/infracast/providers"

func init() {
	_ = providers.DefaultRegistry.Register(&Provider{})
}
