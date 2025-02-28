package keeper

import (
	"time"

	"github.com/NibiruChain/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common/asset"
	epochstypes "github.com/NibiruChain/nibiru/x/epochs/types"
	types "github.com/NibiruChain/nibiru/x/perp/types/v1"
	v2 "github.com/NibiruChain/nibiru/x/perp/types/v2"
)

func (k Keeper) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber uint64) {
}

func (k Keeper) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, _ uint64) {
	for _, market := range k.Markets.Iterate(ctx, collections.Range[asset.Pair]{}).Values() {
		if !market.Enabled || epochIdentifier != market.FundingRateEpochId {
			return
		}

		indexTWAP, err := k.OracleKeeper.GetExchangeRateTwap(ctx, market.Pair)
		if err != nil {
			ctx.Logger().Error("failed to fetch twap index price", "market.Pair", market.Pair, "error", err)
			continue
		}
		if indexTWAP.IsZero() {
			ctx.Logger().Error("index price is zero", "market.Pair", market.Pair)
			continue
		}

		markTwap, err := k.CalcTwap(ctx, market.Pair, v2.TwapCalcOption_SPOT, v2.Direction_DIRECTION_UNSPECIFIED, sdk.ZeroDec(), market.TwapLookbackWindow)
		if err != nil {
			ctx.Logger().Error("failed to fetch twap mark price", "market.Pair", market.Pair, "error", err)
			continue
		}
		if markTwap.IsZero() {
			ctx.Logger().Error("mark price is zero", "market.Pair", market.Pair)
			continue
		}

		epochInfo := k.EpochKeeper.GetEpochInfo(ctx, epochIdentifier)
		intervalsPerDay := (24 * time.Hour) / epochInfo.Duration
		// See https://www.notion.so/nibiru/Funding-Payments-5032d0f8ed164096808354296d43e1fa for an explanation of these terms.
		premiumFraction := markTwap.Sub(indexTWAP).QuoInt64(int64(intervalsPerDay))

		market.LatestCumulativePremiumFraction = market.LatestCumulativePremiumFraction.Add(premiumFraction)
		k.Markets.Insert(ctx, market.Pair, market)

		_ = ctx.EventManager().EmitTypedEvent(&types.FundingRateChangedEvent{
			Pair:                      market.Pair,
			MarkPrice:                 markTwap,
			IndexPrice:                indexTWAP,
			LatestFundingRate:         premiumFraction.Quo(indexTWAP),
			LatestPremiumFraction:     premiumFraction,
			CumulativePremiumFraction: market.LatestCumulativePremiumFraction,
			BlockHeight:               ctx.BlockHeight(),
			BlockTimeMs:               ctx.BlockTime().UnixMilli(),
		})
	}
}

// ___________________________________________________________________________________________________

// Hooks wrapper struct for perps keeper.
type Hooks struct {
	k Keeper
}

var _ epochstypes.EpochHooks = Hooks{}

// Hooks Return the wrapper struct.
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// BeforeEpochStart epochs hooks.
func (h Hooks) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber uint64) {
	h.k.BeforeEpochStart(ctx, epochIdentifier, epochNumber)
}

func (h Hooks) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, epochNumber uint64) {
	h.k.AfterEpochEnd(ctx, epochIdentifier, epochNumber)
}
