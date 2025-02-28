package action

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/app"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/testutil/action"
	"github.com/NibiruChain/nibiru/x/perp/keeper/v2"
	v2types "github.com/NibiruChain/nibiru/x/perp/types/v2"
)

type queryPosition struct {
	pair             asset.Pair
	traderAddress    sdk.AccAddress
	responseCheckers []QueryPositionChecker
}

func (q queryPosition) Do(app *app.NibiruApp, ctx sdk.Context) (sdk.Context, error, bool) {
	queryServer := keeper.NewQuerier(app.PerpKeeperV2)

	resp, err := queryServer.QueryPosition(sdk.WrapSDKContext(ctx), &v2types.QueryPositionRequest{
		Pair:   q.pair,
		Trader: q.traderAddress.String(),
	})
	if err != nil {
		return ctx, err, false
	}

	for _, checker := range q.responseCheckers {
		if err := checker(*resp); err != nil {
			return ctx, err, false
		}
	}

	return ctx, nil, false
}

func QueryPosition(pair asset.Pair, traderAddress sdk.AccAddress, responseCheckers ...QueryPositionChecker) action.Action {
	return queryPosition{
		pair:             pair,
		traderAddress:    traderAddress,
		responseCheckers: responseCheckers,
	}
}

type QueryPositionChecker func(resp v2types.QueryPositionResponse) error

func QueryPosition_PositionEquals(expected v2types.Position) QueryPositionChecker {
	return func(resp v2types.QueryPositionResponse) error {
		return v2types.PositionsAreEqual(&expected, &resp.Position)
	}
}

func QueryPosition_PositionNotionalEquals(expected sdk.Dec) QueryPositionChecker {
	return func(resp v2types.QueryPositionResponse) error {
		if !expected.Equal(resp.PositionNotional) {
			return fmt.Errorf("expected position notional %s, got %s", expected, resp.PositionNotional)
		}
		return nil
	}
}

func QueryPosition_UnrealizedPnlEquals(expected sdk.Dec) QueryPositionChecker {
	return func(resp v2types.QueryPositionResponse) error {
		if !expected.Equal(resp.UnrealizedPnl) {
			return fmt.Errorf("expected unrealized pnl %s, got %s", expected, resp.UnrealizedPnl)
		}
		return nil
	}
}

func QueryPosition_MarginRatioEquals(expected sdk.Dec) QueryPositionChecker {
	return func(resp v2types.QueryPositionResponse) error {
		if !expected.Equal(resp.MarginRatio) {
			return fmt.Errorf("expected margin ratio %s, got %s", expected, resp.MarginRatio)
		}
		return nil
	}
}

type queryAllPositions struct {
	traderAddress       sdk.AccAddress
	allResponseCheckers [][]QueryPositionChecker
}

func (q queryAllPositions) Do(app *app.NibiruApp, ctx sdk.Context) (sdk.Context, error, bool) {
	queryServer := keeper.NewQuerier(app.PerpKeeperV2)

	resp, err := queryServer.QueryPositions(sdk.WrapSDKContext(ctx), &v2types.QueryPositionsRequest{
		Trader: q.traderAddress.String(),
	})
	if err != nil {
		return ctx, err, false
	}

	for i, positionCheckers := range q.allResponseCheckers {
		for _, checker := range positionCheckers {
			if err := checker(resp.Positions[i]); err != nil {
				return ctx, err, false
			}
		}
	}

	return ctx, nil, false
}

func QueryPositions(traderAddress sdk.AccAddress, responseCheckers ...[]QueryPositionChecker) action.Action {
	return queryAllPositions{
		traderAddress:       traderAddress,
		allResponseCheckers: responseCheckers,
	}
}
