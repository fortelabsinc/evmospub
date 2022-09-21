package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgSendOrder = "send_order"

var _ sdk.Msg = &MsgSendOrder{}

func NewMsgSendOrder(
	creator string,
	port string,
	channelID string,
	timeoutTimestamp uint64,
	receiver string,
	instrument string,
	token sdk.Coin,
	direction string,
	price string,
	threshold string,
) *MsgSendOrder {
	return &MsgSendOrder{
		Creator:          creator,
		Port:             port,
		ChannelID:        channelID,
		TimeoutTimestamp: timeoutTimestamp,
		Receiver:         receiver,
		Instrument:       instrument,
		Token:            token,
		Direction:        direction,
		Price:            price,
		Threshold:        threshold,
	}
}

func (msg *MsgSendOrder) Route() string {
	return RouterKey
}

func (msg *MsgSendOrder) Type() string {
	return TypeMsgSendOrder
}

func (msg *MsgSendOrder) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgSendOrder) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSendOrder) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	if msg.Port == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet port")
	}
	if msg.ChannelID == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet channel")
	}
	if msg.TimeoutTimestamp == 0 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet timeout")
	}
	return nil
}
