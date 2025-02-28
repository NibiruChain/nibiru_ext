package keeper

import (
	"testing"
	"time"

	"github.com/NibiruChain/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil/mock"
	"github.com/NibiruChain/nibiru/x/perp/amm/types"
)

func TestSwapQuoteForBase(t *testing.T) {
	tests := []struct {
		name                      string
		pair                      asset.Pair
		direction                 types.Direction
		quoteAmount               sdk.Dec
		baseLimit                 sdk.Dec
		skipFluctuationLimitCheck bool

		expectedQuoteReserve sdk.Dec
		expectedBaseReserve  sdk.Dec
		expectedBaseAmount   sdk.Dec
		expectedErr          error
	}{
		{
			name:                      "quote amount == 0",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			quoteAmount:               sdk.NewDec(0),
			baseLimit:                 sdk.NewDec(10),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve: sdk.NewDec(10 * common.TO_MICRO),
			expectedBaseReserve:  sdk.NewDec(10 * common.TO_MICRO),
			expectedBaseAmount:   sdk.ZeroDec(),
		},
		{
			name:                      "normal swap add",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			quoteAmount:               sdk.NewDec(100_000),
			baseLimit:                 sdk.NewDec(49_504),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve: sdk.NewDec(10_050_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("9950248.756218905472636816"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("49751.243781094527363184"),
		},
		{
			name:                      "normal swap remove",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			quoteAmount:               sdk.NewDec(100_000),
			baseLimit:                 sdk.NewDec(50_506),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve: sdk.NewDec(9_950_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("10050251.256281407035175879"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("50251.256281407035175879"),
		},
		{
			name:                      "base amount less than base limit in Long",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			quoteAmount:               sdk.NewDec(500_000),
			baseLimit:                 sdk.NewDec(454_500),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "base amount more than base limit in Short",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			quoteAmount:               sdk.NewDec(1 * common.TO_MICRO),
			baseLimit:                 sdk.NewDec(454_500),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "over trading limit when removing quote",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			quoteAmount:               sdk.NewDec(21_000_001),
			baseLimit:                 sdk.ZeroDec(),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrQuoteReserveAtZero,
		},
		{
			name:                      "over trading limit when adding quote",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			quoteAmount:               sdk.NewDec(21_000_001),
			baseLimit:                 sdk.ZeroDec(),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverTradingLimit,
		},
		{
			name:                      "over fluctuation limit fails on add",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			quoteAmount:               sdk.NewDec(1 * common.TO_MICRO),
			baseLimit:                 sdk.NewDec(454_544),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit fails on remove",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			quoteAmount:               sdk.NewDec(2 * common.TO_MICRO),
			baseLimit:                 sdk.NewDec(1_555_556),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit allowed on add",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			quoteAmount:               sdk.NewDec(1 * common.TO_MICRO),
			baseLimit:                 sdk.NewDec(454_544),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve: sdk.NewDec(10_500_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("9523809.523809523809523810"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("476190.476190476190476190"),
		},
		{
			name:                      "over fluctuation limit allowed on remove",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			quoteAmount:               sdk.NewDec(1 * common.TO_MICRO),
			baseLimit:                 sdk.NewDec(555_556),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve: sdk.NewDec(9_500_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("10526315.789473684210526316"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("526315.789473684210526316"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oracleKeeper := mock.NewMockOracleKeeper(gomock.NewController(t))
			perpammKeeper, ctx := PerpAmmKeeper(t, oracleKeeper)

			oracleKeeper.EXPECT().GetExchangeRate(gomock.Any(), gomock.Any()).Return(sdk.NewDec(1), nil).AnyTimes()

			assert.NoError(t, perpammKeeper.CreatePool(
				ctx,
				asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				/* quoteReserve */ sdk.NewDec(10*common.TO_MICRO), // 10 tokens
				/* baseReserve */ sdk.NewDec(10*common.TO_MICRO), // 10 tokens
				types.MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"),
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
				},
				sdk.MustNewDecFromStr("2"),
			))
			market, err := perpammKeeper.GetPool(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
			require.NoError(t, err)

			_, baseAmt, err := perpammKeeper.SwapQuoteForBase(
				ctx,
				market,
				tc.direction,
				tc.quoteAmount,
				tc.baseLimit,
				tc.skipFluctuationLimitCheck,
			)

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedBaseAmount, baseAmt, "base amount mismatch")

				market, _ = perpammKeeper.GetPool(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))

				dir := sdk.OneDec()
				if tc.direction == types.Direction_SHORT {
					dir = sdk.OneDec().Neg()
				}

				assert.EqualValuesf(t, dir.Mul(tc.expectedBaseAmount), market.GetBias(), "bias amount mismatch")

				t.Log("assert market")
				pool, err := perpammKeeper.Pools.Get(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedQuoteReserve, pool.QuoteReserve, "pool quote asset reserve mismatch")
				assert.EqualValuesf(t, tc.expectedBaseReserve, pool.BaseReserve, "pool base asset reserve mismatch")
			}
		})
	}
}

func TestSwapBaseForQuote(t *testing.T) {
	tests := []struct {
		name                      string
		pair                      asset.Pair
		direction                 types.Direction
		baseAmt                   sdk.Dec
		quoteLimit                sdk.Dec
		skipFluctuationLimitCheck bool

		expectedQuoteReserve     sdk.Dec
		expectedBaseReserve      sdk.Dec
		expectedQuoteAssetAmount sdk.Dec
		expectedErr              error
	}{
		{
			name:                      "zero base asset swap",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			baseAmt:                   sdk.ZeroDec(),
			quoteLimit:                sdk.ZeroDec(),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve:     sdk.NewDec(10 * common.TO_MICRO),
			expectedBaseReserve:      sdk.NewDec(10 * common.TO_MICRO),
			expectedQuoteAssetAmount: sdk.ZeroDec(),
		},
		{
			name:                      "add base asset swap",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(198_000),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("9900990.099009900990099010"),
			expectedBaseReserve:      sdk.NewDec(10_100_000),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("198019.801980198019801980"),
		},
		{
			name:                      "remove base asset",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(204_082),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("10101010.101010101010101010"),
			expectedBaseReserve:      sdk.NewDec(9_900_000),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("202020.202020202020202020"),
		},
		{
			name:                      "quote amount less than quote limit in Long",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(198_079),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "quote amount more than quote limit in Short",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(201_081),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "over trading limit when removing base",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			baseAmt:                   sdk.NewDec(10_500_001),
			quoteLimit:                sdk.ZeroDec(),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrBaseReserveAtZero,
		},
		{
			name:                      "over trading limit when adding base",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			baseAmt:                   sdk.NewDec(10_500_001),
			quoteLimit:                sdk.ZeroDec(),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverTradingLimit,
		},
		{
			name:                      "over fluctuation limit fails on add",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			baseAmt:                   sdk.NewDec(1 * common.TO_MICRO),
			quoteLimit:                sdk.NewDec(1_666_666),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit fails on remove",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			baseAmt:                   sdk.NewDec(1 * common.TO_MICRO),
			quoteLimit:                sdk.NewDec(2_500_001),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit allowed on add",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_LONG,
			baseAmt:                   sdk.NewDec(1 * common.TO_MICRO),
			quoteLimit:                sdk.NewDec(1_666_666),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("9090909.090909090909090909"),
			expectedBaseReserve:      sdk.NewDec(11 * common.TO_MICRO),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("1818181.818181818181818182"),
		},
		{
			name:                      "over fluctuation limit allowed on remove",
			pair:                      asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			direction:                 types.Direction_SHORT,
			baseAmt:                   sdk.NewDec(1 * common.TO_MICRO),
			quoteLimit:                sdk.NewDec(2_500_001),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("11111111.111111111111111111"),
			expectedBaseReserve:      sdk.NewDec(9 * common.TO_MICRO),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("2222222.222222222222222222"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pfKeeper := mock.NewMockOracleKeeper(gomock.NewController(t))

			perpammKeeper, ctx := PerpAmmKeeper(t, pfKeeper)
			pfKeeper.EXPECT().
				GetExchangeRate(gomock.Any(), gomock.Any()).Return(sdk.NewDec(1), nil).AnyTimes()

			assert.NoError(t, perpammKeeper.CreatePool(
				ctx,
				asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				/* quoteReserve */ sdk.NewDec(10*common.TO_MICRO), // 10 tokens
				/* baseReserve */ sdk.NewDec(10*common.TO_MICRO), // 5 tokens
				types.MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"),
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
				},
				sdk.NewDec(2),
			))

			market, err := perpammKeeper.GetPool(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
			require.NoError(t, err)
			_, quoteAssetAmount, err := perpammKeeper.SwapBaseForQuote(
				ctx,
				market,
				tc.direction,
				tc.baseAmt,
				tc.quoteLimit,
				tc.skipFluctuationLimitCheck,
			)

			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedQuoteAssetAmount, quoteAssetAmount,
					"expected %s; got %s", tc.expectedQuoteAssetAmount.String(), quoteAssetAmount.String())

				t.Log("assert pool")
				market, _ = perpammKeeper.GetPool(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
				dir := sdk.OneDec()
				if tc.direction == types.Direction_LONG {
					dir = sdk.OneDec().Neg()
				}
				assert.EqualValuesf(t, dir.Mul(tc.baseAmt), market.GetBias(), "bias amount mismatch")

				pool, err := perpammKeeper.Pools.Get(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
				require.NoError(t, err)
				assert.Equal(t, tc.expectedQuoteReserve, pool.QuoteReserve)
				assert.Equal(t, tc.expectedBaseReserve, pool.BaseReserve)
			}
		})
	}
}

func TestGetMarkets(t *testing.T) {
	perpammKeeper, ctx := PerpAmmKeeper(t,
		mock.NewMockOracleKeeper(gomock.NewController(t)),
	)

	require.NoError(t, perpammKeeper.CreatePool(
		ctx,
		asset.Registry.Pair(denoms.BTC, denoms.NUSD),
		sdk.NewDec(5*common.TO_MICRO),
		sdk.NewDec(5*common.TO_MICRO),
		types.MarketConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
		sdk.NewDec(2),
	))
	require.NoError(t, perpammKeeper.CreatePool(
		ctx,
		asset.Registry.Pair(denoms.ETH, denoms.NUSD),
		sdk.NewDec(10*common.TO_MICRO),
		sdk.NewDec(10*common.TO_MICRO),
		types.MarketConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
		sdk.MustNewDecFromStr("0.5"),
	))

	pools := perpammKeeper.Pools.Iterate(ctx, collections.Range[asset.Pair]{}).Values()

	require.EqualValues(t, 2, len(pools))

	require.EqualValues(t, pools[0], types.NewMarket(types.ArgsNewMarket{
		Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
		BaseReserves:  sdk.NewDec(5 * common.TO_MICRO),
		QuoteReserves: sdk.NewDec(5 * common.TO_MICRO),
		Config: &types.MarketConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
		TotalLong:     sdk.ZeroDec(),
		TotalShort:    sdk.ZeroDec(),
		PegMultiplier: sdk.NewDec(2),
	}))
	require.EqualValues(t, pools[1], types.NewMarket(types.ArgsNewMarket{
		Pair:          asset.Registry.Pair(denoms.ETH, denoms.NUSD),
		BaseReserves:  sdk.NewDec(10 * common.TO_MICRO),
		QuoteReserves: sdk.NewDec(10 * common.TO_MICRO),
		Config: &types.MarketConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
		TotalLong:     sdk.ZeroDec(),
		TotalShort:    sdk.ZeroDec(),
		PegMultiplier: sdk.MustNewDecFromStr("0.5"),
	}))
}

func TestCheckFluctuationLimitRatio(t *testing.T) {
	tests := []struct {
		name              string
		pool              types.Market
		existingSnapshots []types.ReserveSnapshot

		expectedErr error
	}{
		{
			name: "uses latest snapshot - does not result in error",
			pool: types.Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1_000),
				BaseReserve:   sdk.NewDec(1_000),
				PegMultiplier: sdk.OneDec(),

				Config: types.MarketConfig{
					FluctuationLimitRatio: sdk.MustNewDecFromStr("0.001"),
				},
			},
			existingSnapshots: []types.ReserveSnapshot{
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   0,
					PegMultiplier: sdk.OneDec(),
				},
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   1,
					PegMultiplier: sdk.OneDec(),
				},
			},
			expectedErr: nil,
		},
		{
			name: "uses previous snapshot - results in error",
			pool: types.Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1_100),
				BaseReserve:   sdk.NewDec(900),
				PegMultiplier: sdk.OneDec(),

				Config: types.MarketConfig{
					FluctuationLimitRatio: sdk.MustNewDecFromStr("0.001"),
				},
			},
			existingSnapshots: []types.ReserveSnapshot{
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   0,
					PegMultiplier: sdk.OneDec(),
				},
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   1,
					PegMultiplier: sdk.OneDec(),
				},
			},
			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name: "only one snapshot - no error",
			pool: types.Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1_000),
				BaseReserve:   sdk.NewDec(1_000),
				PegMultiplier: sdk.OneDec(),

				Config: types.MarketConfig{
					FluctuationLimitRatio: sdk.MustNewDecFromStr("0.001"),
				},
			},
			existingSnapshots: []types.ReserveSnapshot{
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   0,
					PegMultiplier: sdk.OneDec(),
				},
			},
			expectedErr: nil,
		},
		{
			name: "zero fluctuation limit - no error",
			pool: types.Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1_100),
				BaseReserve:   sdk.NewDec(900),
				PegMultiplier: sdk.OneDec(),

				Config: types.MarketConfig{
					FluctuationLimitRatio: sdk.ZeroDec(),
				},
			},
			existingSnapshots: []types.ReserveSnapshot{
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   0,
					PegMultiplier: sdk.OneDec(),
				},
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   1,
					PegMultiplier: sdk.OneDec(),
				},
			},
			expectedErr: nil,
		},
		{
			name: "uses latest snapshot - does not result in error",
			pool: types.Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1_000),
				BaseReserve:   sdk.NewDec(1_000),
				PegMultiplier: sdk.OneDec(),

				Config: types.MarketConfig{
					FluctuationLimitRatio: sdk.MustNewDecFromStr("0.001"),
				},
			},
			existingSnapshots: []types.ReserveSnapshot{
				{
					Pair:          asset.Registry.Pair(denoms.ETH, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(5_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   0,
					PegMultiplier: sdk.OneDec(),
				},
				{
					Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(1_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   0,
					PegMultiplier: sdk.OneDec(),
				},
				{
					Pair:          asset.Registry.Pair(denoms.ETH, denoms.NUSD),
					QuoteReserve:  sdk.NewDec(5_000),
					BaseReserve:   sdk.NewDec(1_000),
					TimestampMs:   1,
					PegMultiplier: sdk.OneDec(),
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			perpammKeeper, ctx := PerpAmmKeeper(t,
				mock.NewMockOracleKeeper(gomock.NewController(t)),
			)

			perpammKeeper.Pools.Insert(ctx, tc.pool.Pair, tc.pool)

			for _, snapshot := range tc.existingSnapshots {
				perpammKeeper.ReserveSnapshots.Insert(
					ctx,
					collections.Join(
						snapshot.Pair,
						time.UnixMilli(snapshot.TimestampMs)),
					snapshot)
			}

			t.Log("check fluctuation limit")
			err := perpammKeeper.checkFluctuationLimitRatio(ctx, tc.pool)

			t.Log("check error if any")
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetMaintenanceMarginRatio(t *testing.T) {
	tests := []struct {
		name string
		pool types.Market

		expectedMaintenanceMarginRatio sdk.Dec
	}{
		{
			name: "zero fluctuation limit ratio",
			pool: types.Market{
				Pair:         asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve: sdk.OneDec(),
				BaseReserve:  sdk.OneDec(),
				SqrtDepth:    common.MustSqrtDec(sdk.NewDec(1)),
				Config: *types.DefaultMarketConfig().
					WithMaintenanceMarginRatio(sdk.MustNewDecFromStr("0.9876")),
			},
			expectedMaintenanceMarginRatio: sdk.MustNewDecFromStr("0.9876"),
		},
		{
			name: "zero fluctuation limit ratio",
			pool: types.Market{
				Pair:         asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve: sdk.OneDec(),
				BaseReserve:  sdk.OneDec(),
				SqrtDepth:    common.MustSqrtDec(sdk.NewDec(1)),
				Config: *types.DefaultMarketConfig().
					WithMaintenanceMarginRatio(sdk.MustNewDecFromStr("0.4242")),
			},
			expectedMaintenanceMarginRatio: sdk.MustNewDecFromStr("0.4242"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			perpammKeeper, ctx := PerpAmmKeeper(t,
				mock.NewMockOracleKeeper(gomock.NewController(t)),
			)
			perpammKeeper.Pools.Insert(ctx, tc.pool.Pair, tc.pool)
			mmr, err := perpammKeeper.GetMaintenanceMarginRatio(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
			assert.NoError(t, err)
			assert.EqualValues(t, tc.expectedMaintenanceMarginRatio, mmr)
		})
	}
}
