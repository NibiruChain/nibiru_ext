package ante

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

// BalanceExposerMessageDecorator emits the event with the tx sender balances.
type BalanceExposerMessageDecorator struct {
	ak ante.AccountKeeper
	bk bankkeeper.Keeper
}

func NewBalanceExposerDecorator(ak ante.AccountKeeper, bk authtypes.BankKeeper) BalanceExposerMessageDecorator {
	return BalanceExposerMessageDecorator{
		ak: ak,
		bk: bk.(bankkeeper.Keeper),
	}
}

func (md BalanceExposerMessageDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)

	if !ok {
		return ctx, sdkerrors.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
	}

	for _, addr := range sigTx.GetSigners() {
		balance := md.bk.GetAllBalances(ctx, addr)
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"balance",
				sdk.NewAttribute("address", fmt.Sprintf("%s", addr.String())),
				sdk.NewAttribute("balances", fmt.Sprintf("%s", balance.String())),
			),
		)
	}
	return next(ctx, tx, simulate)
}
