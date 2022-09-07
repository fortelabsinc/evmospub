package keeper

import (
	"github.com/evmos/evmos/v6/x/customtransfer/types"
)

var _ types.QueryServer = Keeper{}
