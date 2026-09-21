//go:build unit

package service

// newStubPricingServiceFromMap 直接注入价格表，绕过价格文件加载。
func newStubPricingServiceFromMap(data map[string]*LiteLLMModelPricing) *PricingService {
	return &PricingService{pricingData: data}
}
