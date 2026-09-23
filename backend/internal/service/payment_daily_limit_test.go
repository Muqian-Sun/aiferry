//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// 每日限额按订单金额（美元）累计：充值单的 Amount 是到账美元、订阅单是套餐价格；
// 网关实付（PayAmount，人民币）不参与累计。
func TestCheckDailyLimitSumsUSDOrderAmounts(t *testing.T) {
	ctx := context.Background()

	db, err := sql.Open("sqlite", "file:payment_daily_limit?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })

	user, err := client.User.Create().SetEmail("daily@example.com").SetPasswordHash("hash").SetUsername("daily").Save(ctx)
	require.NoError(t, err)

	now := time.Now()
	for i, o := range []struct {
		orderType string
		amountUSD float64
		payCNY    float64
	}{
		{payment.OrderTypeBalance, 10, 72},
		{payment.OrderTypeSubscription, 9.9, 71.28},
	} {
		_, err := client.PaymentOrder.Create().
			SetUserID(user.ID).
			SetUserEmail(user.Email).
			SetUserName(user.Username).
			SetAmount(o.amountUSD).
			SetPayAmount(o.payCNY).
			SetFeeRate(0).
			SetRechargeCode("DAILY-LIMIT-" + string(rune('A'+i))).
			SetOutTradeNo("sub2_daily_limit_" + string(rune('a'+i))).
			SetPaymentType(payment.TypeAlipay).
			SetPaymentTradeNo("").
			SetOrderType(o.orderType).
			SetStatus(OrderStatusCompleted).
			SetPaidAt(now).
			SetExpiresAt(now.Add(time.Hour)).
			SetClientIP("127.0.0.1").
			SetSrcHost("api.example.com").
			Save(ctx)
		require.NoError(t, err)
	}

	svc := &PaymentService{}
	check := func(amountUSD float64) error {
		tx, err := client.Tx(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		return svc.checkDailyLimit(ctx, tx, user.ID, amountUSD, 30)
	}

	// 已用 $10 + $9.9 = $19.9；再下 $10 刚好不超 $30
	require.NoError(t, check(10))

	// 再下 $10.2 超限，剩余额度按美元算：$30 - $19.9 = $10.10
	err = check(10.2)
	require.Error(t, err)
	require.Equal(t, "DAILY_LIMIT_EXCEEDED", infraerrors.Reason(err))
	require.Equal(t, "10.10", infraerrors.FromError(err).Metadata["remaining"])
}
