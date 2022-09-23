package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// AccountKeeper defines the expected account keeper used for simulations (no alias)
type AccountKeeper interface {
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) types.AccountI
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	BlockedAddr(addr sdk.AccAddress) bool
	// Methods imported from bank should be defined here
}

type TransferKeeper interface {
	HasDenomTrace(ctx sdk.Context, denomTraceHash tmbytes.HexBytes) bool
	SetDenomTrace(ctx sdk.Context, denomTrace ibctransfertypes.DenomTrace)
	GetDenomTrace(ctx sdk.Context, denomTraceHash tmbytes.HexBytes) (ibctransfertypes.DenomTrace, bool)
	GetReceiveEnabled(ctx sdk.Context) bool
	GetSendEnabled(ctx sdk.Context) bool
	// Methods imported from bank should be defined here
}
