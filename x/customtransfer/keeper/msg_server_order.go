package keeper

import (
	"context"
	"fmt"
	"strconv"

	"github.com/evmos/evmos/v6/x/customtransfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
)

func (k msgServer) SendOrder(goCtx context.Context, msg *types.MsgSendOrder) (*types.MsgSendOrderResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	log := ctx.Logger()
	// TODO: logic before transmitting the packet
	sender, err := sdk.AccAddressFromBech32(msg.Senderaddress)
	if err != nil {
		return nil, err
	}

	amount, _ := strconv.ParseInt(msg.Token.Amount.String(), 0, 32)
	log.Info("**** Burning or locking the coins ***")
	log.Info(fmt.Sprintf("**** Receiver %s", msg.Receiver))

	if err := k.SafeBurn(ctx, msg.Port, msg.ChannelID, sender ,msg.Token.Denom, amount)
	err != nil {
		return nil, err
	}

	if !isIBCToken(msg.Token.Denom) {
		k.SaveVoucherDenom(ctx, msg.Port, msg.ChannelID, msg.Token.Denom)
	}
	// Construct the packet
	var packet types.OrderPacketData

	packet.Receiver = msg.Receiver
	packet.Pair = msg.Pair
	packet.Amount = msg.Token.Amount.String()
	packet.Denom = getDenom(msg.Token.Denom)
	packet.Direction = msg.Direction
	packet.Price = msg.Price
	packet.Threshold = msg.Threshold
	packet.Senderaddress = msg.Senderaddress

	// Transmit the packet
	err = k.TransmitOrderPacket(
		ctx,
		packet,
		msg.Token,
		msg.Port,
		msg.ChannelID,
		clienttypes.ZeroHeight(),
		msg.TimeoutTimestamp,
	)
	if err != nil {
		log.Info("TRASMITTED THE PACKET FAILED")
		return nil, err
	}
	log.Info("TRASMITTED THE PACKET DONE")
	return &types.MsgSendOrderResponse{}, nil
}

func getDenom(denom string) string {
	if isIBCToken(denom) {
		return "src"
	} else {
		return "mud"
	}
}