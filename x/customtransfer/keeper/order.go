package keeper

import (
	"errors"
	"fmt"
	"github.com/evmos/evmos/v6/x/customtransfer/types"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"

	// ibctransferkeeper "github.com/cosmos/ibc-go/v3/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
)

// TransmitOrderPacket transmits the packet over IBC with the specified source port and source channel

// SendTransfer handles transfer sending logic. There are 2 possible cases:
//
// 1. Sender chain is acting as the source zone. The coins are transferred
// to an escrow address (i.e locked) on the sender chain and then transferred
// to the receiving chain through IBC TAO logic. It is expected that the
// receiving chain will mint vouchers to the receiving address.
//
// 2. Sender chain is acting as the sink zone. The coins (vouchers) are burned
// on the sender chain and then transferred to the receiving chain though IBC
// TAO logic. It is expected that the receiving chain, which had previously
// sent the original denomination, will unescrow the fungible token and send
// it to the receiving address.
//
// Another way of thinking of source and sink zones is through the token's
// timeline. Each send to any chain other than the one it was previously
// received from is a movement forwards in the token's timeline. This causes
// trace to be added to the token's history and the destination port and
// destination channel to be prefixed to the denomination. In these instances
// the sender chain is acting as the source zone. When the token is sent back
// to the chain it previously received from, the prefix is removed. This is
// a backwards movement in the token's timeline and the sender chain
// is acting as the sink zone.
//
// Example:
// These steps of transfer occur: A -> B -> C -> A -> C -> B -> A
//
// 1. A -> B : sender chain is source zone. Denom upon receiving: 'B/denom'
// 2. B -> C : sender chain is source zone. Denom upon receiving: 'C/B/denom'
// 3. C -> A : sender chain is source zone. Denom upon receiving: 'A/C/B/denom'
// 4. A -> C : sender chain is sink zone. Denom upon receiving: 'C/B/denom'
// 5. C -> B : sender chain is sink zone. Denom upon receiving: 'B/denom'
// 6. B -> A : sender chain is sink zone. Denom upon receiving: 'denom'

func (k Keeper) TransmitOrderPacket(
	ctx sdk.Context,
	packetData types.OrderPacketData,
	token sdk.Coin,
	sourcePort,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) error {
	// ibc code we can ignore start
	sourceChannelEnd, found := k.ChannelKeeper.GetChannel(ctx, sourcePort, sourceChannel)
	if !found {
		return sdkerrors.Wrapf(channeltypes.ErrChannelNotFound, "port ID (%s) channel ID (%s)", sourcePort, sourceChannel)
	}

	destinationPort := sourceChannelEnd.GetCounterparty().GetPortID()
	destinationChannel := sourceChannelEnd.GetCounterparty().GetChannelID()

	// get the next sequence
	sequence, found := k.ChannelKeeper.GetNextSequenceSend(ctx, sourcePort, sourceChannel)
	if !found {
		return sdkerrors.Wrapf(
			channeltypes.ErrSequenceSendNotFound,
			"source port: %s, source channel: %s", sourcePort, sourceChannel,
		)
	}

	channelCap, ok := k.ScopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(sourcePort, sourceChannel))
	if !ok {
		return sdkerrors.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}
	
	packetBytes, err := packetData.GetBytes()

	if err != nil {
		return sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, "cannot marshal the packet: "+err.Error())
	}

	packet := channeltypes.NewPacket(
		packetBytes,
		sequence,
		sourcePort,
		sourceChannel,
		destinationPort,
		destinationChannel,
		timeoutHeight,
		timeoutTimestamp,
	)


	if err := k.ChannelKeeper.SendPacket(ctx, channelCap, packet); err != nil {
		return err
	}
	return nil
}

// OnRecvOrderPacket processes packet reception

// OnRecvPacket processes a cross chain fungible token transfer. If the
// sender chain is the source of minted tokens then vouchers will be minted
// and sent to the receiving address. Otherwise if the sender chain is sending
// back tokens this chain originally transferred to it, the tokens are
// unescrowed and sent to the receiving address.
func (k Keeper) OnRecvOrderPacket(ctx sdk.Context, packet channeltypes.Packet, data types.OrderPacketData) (packetAck types.OrderPacketAck, err error) {
	log := ctx.Logger()

	log.Info(fmt.Sprintf("CustomTransfer module: arrived order %v", data))
	log.Info(fmt.Sprintf("CustomTransfer module: arrived packet %v", packet))


	// validate packet data upon receiving
	if err := data.ValidateBasic(); err != nil {
		return packetAck, err
	}

	// if !k.transferKeeper.GetReceiveEnabled(ctx) {
	// 	return packetAck, ibctransfertypes.ErrReceiveDisabled
	// }

	finalAmountDenom, saved := k.OriginalDenom(ctx, packet.DestinationPort, packet.DestinationChannel, data.Denom)
	if !saved {
		// If it was not from this chain we use voucher as denom
		finalAmountDenom = VoucherDenom(ctx, packet.SourcePort, packet.SourceChannel, data.Denom)
		k.SaveVoucherDenom(ctx, packet.SourcePort, packet.SourceChannel, data.Denom)
	}
	
	log.Info(fmt.Sprintf("finalAmountDenom %s", finalAmountDenom))
	// Dispatch liquidated swapping orders
	// decode the receiver address
	receiver, err := sdk.AccAddressFromBech32(data.Receiver)
	if err != nil {
		return packetAck, err
	}

	amount, _ := strconv.ParseInt(data.Amount, 0, 32)
	if err := k.SafeMint(
		ctx,
		packet.DestinationPort,
		packet.DestinationChannel,
		receiver,
		finalAmountDenom,
		amount,
	); err != nil {
		return packetAck, err
	}

	// Add the order list here maybe to confirm that order was received

	return packetAck, err	
}

// OnAcknowledgementOrderPacket responds to the the success or failure of a packet
// acknowledgement written on the receiving chain.

// OnAcknowledgementPacket responds to the the success or failure of a packet
// acknowledgement written on the receiving chain. If the acknowledgement
// was a success then nothing occurs. If the acknowledgement failed, then
// the sender is refunded their tokens using the refundPacketToken function.
func (k Keeper) OnAcknowledgementOrderPacket(ctx sdk.Context, packet channeltypes.Packet, data types.OrderPacketData, ack channeltypes.Acknowledgement) error {
	switch dispatchedAck := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Error:
		return k.refundPacketToken(ctx, packet, data)
		// TODO: failed acknowledgement logic
		// _ = dispatchedAck.Error

		// return nil
	case *channeltypes.Acknowledgement_Result:
		// Decode the packet acknowledgment
		var packetAck types.OrderPacketAck

		if err := types.ModuleCdc.UnmarshalJSON(dispatchedAck.Result, &packetAck); err != nil {
			// The counter-party module doesn't implement the correct acknowledgment format
			return errors.New("cannot unmarshal acknowledgment")
		}

		// TODO: successful acknowledgement logic

		return nil
	default:
		// The counter-party module doesn't implement the correct acknowledgment format
		return errors.New("invalid acknowledgment format")
	}
}

// OnTimeoutOrderPacket responds to the case where a packet has not been transmitted because of a timeout

// OnTimeoutPacket refunds the sender since the original packet sent was
// never received and has been timed out.
func (k Keeper) OnTimeoutOrderPacket(ctx sdk.Context, packet channeltypes.Packet, data types.OrderPacketData) error {
	return k.refundPacketToken(ctx, packet, data)
}

// refundPacketToken will unescrow and send back the tokens back to sender
// if the sending chain was the source chain. Otherwise, the sent tokens
// were burnt in the original send so new tokens are minted and sent to
// the sending address.
func (k Keeper) refundPacketToken(ctx sdk.Context, packet channeltypes.Packet, data types.OrderPacketData) error {
	// NOTE: packet data type already checked in handler.go

	// parse the denomination from the full denom path
	trace := ibctransfertypes.ParseDenomTrace(data.Denom)

	// parse the transfer amount
	transferAmount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		return sdkerrors.Wrapf(ibctransfertypes.ErrInvalidAmount, "unable to parse transfer amount (%s) into sdk.Int", data.Amount)
	}
	token := sdk.NewCoin(trace.IBCDenom(), transferAmount)

	// decode the sender address
	sender, err := sdk.AccAddressFromBech32(data.Senderaddress)
	if err != nil {
		return err
	}

	if ibctransfertypes.SenderChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), data.Denom) {
		// unescrow tokens back to sender
		escrowAddress := ibctransfertypes.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())
		if err := k.bankKeeper.SendCoins(ctx, escrowAddress, sender, sdk.NewCoins(token)); err != nil {
			// NOTE: this error is only expected to occur given an unexpected bug or a malicious
			// counterparty module. The bug may occur in bank or any part of the code that allows
			// the escrow address to be drained. A malicious counterparty module could drain the
			// escrow address by allowing more tokens to be sent back then were escrowed.
			return sdkerrors.Wrap(err, "unable to unescrow tokens, this may be caused by a malicious counterparty module or a bug: please open an issue on counterparty module")
		}

		return nil
	}

	// mint vouchers back to sender
	if err := k.bankKeeper.MintCoins(
		ctx, types.ModuleName, sdk.NewCoins(token),
	); err != nil {
		return err
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, sdk.NewCoins(token)); err != nil {
		panic(fmt.Sprintf("unable to send coins from module to account despite previously minting coins to module account: %v", err))
	}

	return nil
}

// DenomPathFromHash returns the full denomination path prefix from an ibc denom with a hash
// component.
func (k Keeper) DenomPathFromHash(ctx sdk.Context, denom string) (string, error) {
	// trim the denomination prefix, by default "ibc/"
	log := ctx.Logger()
	hexHash := denom[len(ibctransfertypes.DenomPrefix+"/"):]

	hash, err := ibctransfertypes.ParseHexHash(hexHash)
	log.Info(fmt.Sprintf("LLL***************** PARSING START DENOM %s", denom))
	log.Info(fmt.Sprintf("LLL***************** PARSING START HASH %v", hash))

	if err != nil {
		log.Info("LLL***************** PARSING ERROR 1")
		return "", sdkerrors.Wrap(ibctransfertypes.ErrInvalidDenomForTransfer, err.Error())
	}
	log.Info("LLL***************** PARSING GETTING DENOM TRACE")

	denomTrace, found := k.transferKeeper.GetDenomTrace(ctx, hash)

	log.Info("LLL***************** PARSING START 2 %v", denomTrace)

	if !found {
		log.Info("LLL***************** PARSING ERROR 2 NOT FOUND")
		return "", sdkerrors.Wrap(ibctransfertypes.ErrTraceNotFound, hexHash)
	}
	log.Info("LLL***************** PARSING SUCCESS")

	fullDenomPath := denomTrace.GetFullDenomPath()
	return fullDenomPath, nil
}

