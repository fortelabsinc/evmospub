package keeper

import (
	"errors"
	"fmt"
	"github.com/evmos/evmos/v6/x/customtransfer/types"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	coretypes "github.com/cosmos/ibc-go/v3/modules/core/types"

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
	log := ctx.Logger()

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

	// begin createOutgoingPacket logic
	// See spec for this logic: https://github.com/cosmos/ibc/tree/master/spec/app/ics-020-fungible-token-transfer#packet-relay
	channelCap, ok := k.ScopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(sourcePort, sourceChannel))
	if !ok {
		return sdkerrors.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}

	// NOTE: denomination and hex hash correctness checked during msg.ValidateBasic
	fullDenomPath := packetData.Denom //could be packetData.token.Denom (if we send sdk.Coin as token within packetData)

	
	var err error
	log.Info("*************FULL DENOM FETCH START**********")
	// deconstruct the token denomination into the denomination trace info
	// to determine if the sender is the source chain
	if isIBCToken(packetData.Denom) {
		log.Info("LLL*************ITS AN IBC TOKEN**********")
		fullDenomPath, err = k.DenomPathFromHash(ctx, packetData.Denom)
		if err != nil {
			log.Info("LLL*************FULL DENOM ERROR**********")
			return err
		}
	} else {
		log.Info("LLL**** NOT IBC TOKEN")
	}

	labels := []metrics.Label{
		telemetry.NewLabel(coretypes.LabelDestinationPort, destinationPort),
		telemetry.NewLabel(coretypes.LabelDestinationChannel, destinationChannel),
	}

	// NOTE: SendTransfer simply sends the denomination as it exists on its own
	// chain inside the packet data. The receiving chain will perform denom
	// prefixing as necessary.

	// decode the sender address
	sender, err := sdk.AccAddressFromBech32(packetData.Senderaddress)
	if err != nil {
		return err
	}

	
	log.Info("LLL*************IN IF**********")
	log.Info("LLL**************VALIDATE THINGS HERE**********")
	log.Info(fmt.Sprintf("LLL*fullDenomPath %s", fullDenomPath))
	log.Info("LLL**************END IF**********")

	if ibctransfertypes.SenderChainIsSource(sourcePort, sourceChannel, fullDenomPath) {
		labels = append(labels, telemetry.NewLabel(coretypes.LabelSource, "true"))

		// create the escrow address for the tokens
		escrowAddress := ibctransfertypes.GetEscrowAddress(sourcePort, sourceChannel)

		log.Info("LLL**************ESCROW**************")
		log.Info("LLL**************** %s", escrowAddress)
		log.Info("LLL**************ESCROW**************")
		
		log.Info(fmt.Sprintf("LLL*ADDDDRRRRRR1 %v", packetData.Senderaddress))
		log.Info(fmt.Sprintf("LLL*ADDDDRRRRRR2 %v", sender))
		log.Info(fmt.Sprintf("LLL*ADDDDRRRRRR3 %v", k.bankKeeper.GetBalance(ctx, sender, token.Denom)))
		// escrow source tokens. It fails if balance insufficient.
		if err := k.bankKeeper.SendCoins(
			ctx, sender, escrowAddress, sdk.NewCoins(token),
		); err != nil {
			return err
		}

	} else {
		log.Info("LLL**************SENDER CHAIN IS NOT SOURCE**************")
		log.Info("LLL**************SENDER CHAIN IS NOT SOURCE**************")

		labels = append(labels, telemetry.NewLabel(coretypes.LabelSource, "false"))

		// transfer the coins to the module account and burn them
		if err := k.bankKeeper.SendCoinsFromAccountToModule(
			ctx, sender, types.ModuleName, sdk.NewCoins(token),
		); err != nil {
			return err
		}

		if err := k.bankKeeper.BurnCoins(
			ctx, types.ModuleName, sdk.NewCoins(token),
		); err != nil {
			// NOTE: should not happen as the module account was
			// retrieved on the step above and it has enough balace
			// to burn.
			panic(fmt.Sprintf("cannot burn coins after a successful send to a module account: %v", err))
		}
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

	defer func() {
		if token.Amount.IsInt64() {
			telemetry.SetGaugeWithLabels(
				[]string{"tx", "msg", "ibc", "transfer"},
				float32(token.Amount.Int64()),
				[]metrics.Label{telemetry.NewLabel(coretypes.LabelDenom, fullDenomPath)},
			)
		}

		telemetry.IncrCounterWithLabels(
			[]string{"ibc", types.ModuleName, "send"},
			1,
			labels,
		)
	}()

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

	log.Info(fmt.Sprintf("Arrived Order %v", data))
	
	// validate packet data upon receiving
	if err := data.ValidateBasic(); err != nil {
		return packetAck, err
	}
	
	log.Info(fmt.Sprintf("Arrived Order 233 %v", data))
	if !k.transferKeeper.GetReceiveEnabled(ctx) {
		log.Info("RECEIVE NOT ENABLED")
		return packetAck, ibctransfertypes.ErrReceiveDisabled
	}
	log.Info("Arrived order 2")

	// decode the receiver address
	receiver, err := sdk.AccAddressFromBech32(data.Receiver)
	if err != nil {
		return packetAck, err
	}
	log.Info("Arrived order 22")
	// parse the transfer amount
	transferAmount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		return packetAck, sdkerrors.Wrapf(ibctransfertypes.ErrInvalidAmount, "unable to parse transfer amount (%s) into sdk.Int", data.Amount)
	}

	labels := []metrics.Label{
		telemetry.NewLabel(coretypes.LabelSourcePort, packet.GetSourcePort()),
		telemetry.NewLabel(coretypes.LabelSourceChannel, packet.GetSourceChannel()),
	}
	log.Info(fmt.Sprintf("Arrived Order 4 %v", data))
	// This is the prefix that would have been prefixed to the denomination
	// on sender chain IF and only if the token originally came from the
	// receiving chain.
	//
	// NOTE: We use SourcePort and SourceChannel here, because the counterparty
	// chain would have prefixed with DestPort and DestChannel when originally
	// receiving this coin as seen in the "sender chain is the source" condition.

	if ibctransfertypes.ReceiverChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), data.Denom) {
		// sender chain is not the source, unescrow tokens
		log.Info(fmt.Sprintf("Arrived Order 4 %v", packet))
		// remove prefix added by sender chain
		voucherPrefix := ibctransfertypes.GetDenomPrefix(packet.GetSourcePort(), packet.GetSourceChannel())
		unprefixedDenom := data.Denom[len(voucherPrefix):]

		// coin denomination used in sending from the escrow address
		denom := unprefixedDenom

		// The denomination used to send the coins is either the native denom or the hash of the path
		// if the denomination is not native.
		denomTrace := ibctransfertypes.ParseDenomTrace(unprefixedDenom)
		log.Info(fmt.Sprintf("Arrived Order 5 denomTrace %v", unprefixedDenom))
		if denomTrace.Path != "" {
			denom = denomTrace.IBCDenom()
		}
		token := sdk.NewCoin(denom, transferAmount)

		if k.bankKeeper.BlockedAddr(receiver) {
			return packetAck, sdkerrors.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive funds", receiver)
		}

		// unescrow tokens
		escrowAddress := ibctransfertypes.GetEscrowAddress(packet.GetDestPort(), packet.GetDestChannel())
		if err := k.bankKeeper.SendCoins(ctx, escrowAddress, receiver, sdk.NewCoins(token)); err != nil {
			// NOTE: this error is only expected to occur given an unexpected bug or a malicious
			// counterparty module. The bug may occur in bank or any part of the code that allows
			// the escrow address to be drained. A malicious counterparty module could drain the
			// escrow address by allowing more tokens to be sent back then were escrowed.
			return packetAck, sdkerrors.Wrap(err, "unable to unescrow tokens, this may be caused by a malicious counterparty module or a bug: please open an issue on counterparty module")
		}

		defer func() {
			if transferAmount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"ibc", types.ModuleName, "packet", "receive"},
					float32(transferAmount.Int64()),
					[]metrics.Label{telemetry.NewLabel(coretypes.LabelDenom, unprefixedDenom)},
				)
			}

			telemetry.IncrCounterWithLabels(
				[]string{"ibc", types.ModuleName, "receive"},
				1,
				append(
					labels, telemetry.NewLabel(coretypes.LabelSource, "true"),
				),
			)
		}()

		log.Info(fmt.Sprintf("Arrived Order 6 escrowAddress %s", escrowAddress))
		return packetAck, nil
	}

	// sender chain is the source, mint vouchers

	// since SendPacket did not prefix the denomination, we must prefix denomination here
	sourcePrefix := ibctransfertypes.GetDenomPrefix(packet.GetDestPort(), packet.GetDestChannel())
	// NOTE: sourcePrefix contains the trailing "/"
	prefixedDenom := sourcePrefix + data.Denom

	log.Info("LLL* Starting the denom trace setup")
	
	// construct the denomination trace from the full raw denomination
	denomTrace := ibctransfertypes.ParseDenomTrace(prefixedDenom)

	log.Info(fmt.Sprintf("LLL* denomTrace ->  %v", denomTrace))
	
	traceHash := denomTrace.Hash()

	log.Info(fmt.Sprintf("LLL* traceHash ->  %v", traceHash))
	
	log.Info(fmt.Sprintf("LLL* denomTrace %v", denomTrace))

	log.Info(fmt.Sprintf("LLL* GOING TO SET DENOM TRACE ->  %v", traceHash))
	if !k.transferKeeper.HasDenomTrace(ctx, traceHash) {
		log.Info(fmt.Sprintf("LLL* STARTING TO SET DENOM TRACE ->  %v", traceHash))
		k.transferKeeper.SetDenomTrace(ctx, denomTrace)
	}

	voucherDenom := denomTrace.IBCDenom()
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			ibctransfertypes.EventTypeDenomTrace,
			sdk.NewAttribute(ibctransfertypes.AttributeKeyTraceHash, traceHash.String()),
			sdk.NewAttribute(ibctransfertypes.AttributeKeyDenom, voucherDenom),
		),
	)
	voucher := sdk.NewCoin(voucherDenom, transferAmount)

	// mint new tokens if the source of the transfer is the same chain
	if err := k.bankKeeper.MintCoins(
		ctx, types.ModuleName, sdk.NewCoins(voucher),
	); err != nil {
		return packetAck, err
	}

	// send to receiver
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, receiver, sdk.NewCoins(voucher),
	); err != nil {
		return packetAck, err
	}

	defer func() {
		if transferAmount.IsInt64() {
			telemetry.SetGaugeWithLabels(
				[]string{"ibc", types.ModuleName, "packet", "receive"},
				float32(transferAmount.Int64()),
				[]metrics.Label{telemetry.NewLabel(coretypes.LabelDenom, data.Denom)},
			)
		}

		telemetry.IncrCounterWithLabels(
			[]string{"ibc", types.ModuleName, "receive"},
			1,
			append(
				labels, telemetry.NewLabel(coretypes.LabelSource, "false"),
			),
		)
	}()

	// TODO: packet reception logic

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
	hexHash := "C895F3BFC922EAF1428A10D4DBF530D253EA986760B0D29A09670F6F5A4E62A8"

	hash, err := ibctransfertypes.ParseHexHash(hexHash)
	log.Info(fmt.Sprintf("LLL***************** PARSING START DENOM %s", denom))
	log.Info(fmt.Sprintf("LLL***************** PARSING START HASH %s", hash))

	if err != nil {
		log.Info("LLL***************** PARSING ERROR 1")
		return "", sdkerrors.Wrap(ibctransfertypes.ErrInvalidDenomForTransfer, err.Error())
	}
	log.Info("LLL***************** PARSING GETTING DENOM TRACE")

	denomTrace, found := k.transferKeeper.GetDenomTrace(ctx, hash)

	log.Info("LLL***************** PARSING START 2 %s", denomTrace)

	if !found {
		log.Info("LLL***************** PARSING ERROR 2 NOT FOUND")
		return "", sdkerrors.Wrap(ibctransfertypes.ErrTraceNotFound, hexHash)
	}
	log.Info("LLL***************** PARSING SUCCESS")

	fullDenomPath := denomTrace.GetFullDenomPath()
	return fullDenomPath, nil
}

func isIBCToken(denom string) bool {
  return strings.HasPrefix(denom, "ibc/")
}

// func (k Keeper) SaveVoucherDenom(ctx sdk.Context, port string, channel string, denom string) {
// 	voucher := VoucherDenom(port, channel, denom)

// 	// Store the origin denom
// 	_, saved := k.transferKeeper.GetDenomTrace(ctx, voucher)
// 	if !saved {
// 			k.transferKeeper.SetDenomTrace(ctx, ibctransfertypes.DenomTrace{
					
// 			})
// 	}
// }

// x/dex/keeper/denom.go

func VoucherDenom(port string, channel string, denom string) string {
  // since SendPacket did not prefix the denomination, we must prefix denomination here
  sourcePrefix := ibctransfertypes.GetDenomPrefix(port, channel)

  // NOTE: sourcePrefix contains the trailing "/"
  prefixedDenom := sourcePrefix + denom

  // construct the denomination trace from the full raw denomination
  denomTrace := ibctransfertypes.ParseDenomTrace(prefixedDenom)
  voucher := denomTrace.IBCDenom()
  return voucher[:16]
}
