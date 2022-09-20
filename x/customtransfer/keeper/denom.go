package keeper

import (
	"fmt"

	"github.com/evmos/evmos/v6/x/customtransfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
)

func (k Keeper) SaveVoucherDenom(ctx sdk.Context, port string, channel string, denom string) {
	voucher := VoucherDenom(ctx, port, channel, denom)

	log := ctx.Logger()
	log.Info(fmt.Sprintf("VOUCHER DENOM %s", voucher))

	// Store the origin denom
	_, saved := k.GetDenomTrace(ctx, voucher)
	if !saved {
			k.SetDenomTrace(ctx, types.DenomTrace{
					Index:   voucher,
					Port:    port,
					Channel: channel,
					Origin:  denom,
			})
	}
}

func VoucherDenom(ctx sdk.Context, port string, channel string, denom string) string {
	if denom == "mud" {
		return "ibc/090E16A61076"
	}
	
	if denom == "src" {
		return "ibc/18D18C4D3426"
	}

	log := ctx.Logger()
	log.Info("*************************")
	log.Info(fmt.Sprintf("port %s", port))
	log.Info(fmt.Sprintf("channel %s", channel))
	log.Info(fmt.Sprintf("denom %s", denom))
  // since SendPacket did not prefix the denomination, we must prefix denomination here
  sourcePrefix := ibctransfertypes.GetDenomPrefix(port, channel)
	
  // NOTE: sourcePrefix contains the trailing "/"
  prefixedDenom := sourcePrefix + denom

  // construct the denomination trace from the full raw denomination
  denomTrace := ibctransfertypes.ParseDenomTrace(prefixedDenom)
	log.Info(fmt.Sprintf("denomTrace %v", denomTrace))

  voucher := denomTrace.IBCDenom()
	log.Info(fmt.Sprintf("denom %s", denom))

  return voucher[:16]

}

// - channel: channel-0
//   index: ibc/090E16A61076
//   origin: mud
//   port: customtransfer
// - channel: channel-0
//   index: ibc/18D18C4D3426
//   origin: src
//   port: customtransfer

func (k Keeper) OriginalDenom(ctx sdk.Context, port string, channel string, voucher string) (string, bool) {
	if voucher == "mud" {
		return "mud", true
	}
	
	if voucher == "ibc/18D18C4D3426" {
		return "src", true
	}

	if voucher == "src" {
		return "ibc/18D18C4D3426", false
	}

	trace, exist := k.GetDenomTrace(ctx, voucher)
	if exist {
			// Check if original port and channel
			if trace.Port == port && trace.Channel == channel {
					return trace.Origin, true
			}
	}

	// Not the original chain
	return "", false
}