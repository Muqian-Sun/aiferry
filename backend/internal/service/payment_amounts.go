package service

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// normalizeUSDToCNYRate 将非法值归一为 0（未配置）。
func normalizeUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

// convertUSDToGatewayAmount 把美元订单金额（充值到账额 / 订阅价格）换算成网关扣款基数（不含手续费）。
// USD 通道原价；CNY 通道 × 美元汇率，汇率未配置时拒绝下单——不把美元数字当人民币收；
// 其他币种没有可用汇率，拒绝下单。
func convertUSDToGatewayAmount(amountUSD, usdToCNYRate float64, currency string) (float64, error) {
	switch currency {
	case payment.USDPaymentCurrency:
		return amountUSD, nil
	case payment.DefaultPaymentCurrency:
		rate := normalizeUSDToCNYRate(usdToCNYRate)
		if rate <= 0 {
			return 0, infraerrors.ServiceUnavailable("USD_TO_CNY_RATE_NOT_CONFIGURED", "USD to CNY rate is not configured")
		}
		return decimal.NewFromFloat(amountUSD).
			Mul(decimal.NewFromFloat(rate)).
			Round(int32(payment.CurrencyMaxFractionDigits(currency))).
			InexactFloat64(), nil
	default:
		return 0, infraerrors.ServiceUnavailable("UNSUPPORTED_PAYMENT_CURRENCY", "only CNY and USD payment channels are supported").
			WithMetadata(map[string]string{"currency": currency})
	}
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
