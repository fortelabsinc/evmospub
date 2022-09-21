package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/evmos/evmos/v6/x/customtransfer/types"
)

func (k msgServer) SendOrder(goCtx context.Context, msg *types.MsgSendOrder) (*types.MsgSendOrderResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: logic before transmitting the packet

	// Construct the packet
	var packet types.OrderPacketData

	packet.Receiver = msg.Receiver
	packet.Instrument = msg.Instrument
	packet.Denom = msg.Token.Denom
	packet.Amount = msg.Token.Amount.String()
	packet.Direction = msg.Direction
	packet.Price = msg.Price
	packet.Threshold = msg.Threshold
	packet.Senderaddress = msg.Creator

	// Transmit the packet
	err := k.TransmitOrderPacket(
		ctx,
		packet,
		msg.Token,
		msg.Port,
		msg.ChannelID,
		clienttypes.ZeroHeight(),
		msg.TimeoutTimestamp,
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgSendOrderResponse{}, nil
}
