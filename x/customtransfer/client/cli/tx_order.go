package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channelutils "github.com/cosmos/ibc-go/v3/modules/core/04-channel/client/utils"
	"github.com/spf13/cobra"
	"github.com/evmos/evmos/v6/x/customtransfer/types"
)

var _ = strconv.Itoa(0)

func CmdSendOrder() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-order [src-port] [src-channel] [receiver] [instrument] [token] [direction] [threshold]",
		Short: "Send a order over IBC",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			creator := clientCtx.GetFromAddress().String()
			srcPort := args[0]
			srcChannel := args[1]

			argReceiver := args[2]
			argInstrument := args[3]
			argToken, err := sdk.ParseCoinNormalized(args[4])
			if err != nil {
				return err
			}
			argDirection := args[5]
			argThreshold := args[6]

			// Get the relative timeout timestamp
			timeoutTimestamp, err := cmd.Flags().GetUint64(flagPacketTimeoutTimestamp)
			if err != nil {
				return err
			}
			consensusState, _, _, err := channelutils.QueryLatestConsensusState(clientCtx, srcPort, srcChannel)
			if err != nil {
				return err
			}
			if timeoutTimestamp != 0 {
				timeoutTimestamp = consensusState.GetTimestamp() + timeoutTimestamp
			}

			msg := types.NewMsgSendOrder(creator, srcPort, srcChannel, timeoutTimestamp, argReceiver, argInstrument, argToken, argDirection, argThreshold)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64(flagPacketTimeoutTimestamp, DefaultRelativePacketTimeoutTimestamp, "Packet timeout timestamp in nanoseconds. Default is 10 minutes.")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
