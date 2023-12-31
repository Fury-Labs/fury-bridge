package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"

	"github.com/fury-labs/fury-bridge/contract"
	"github.com/fury-labs/fury-bridge/x/bridge/types"
)

// Hooks wrapper struct for bridge keeper
type ConversionHooks struct {
	k Keeper
}

var _ evmtypes.EvmHooks = ConversionHooks{}

// Return the wrapper struct
func (k Keeper) ConversionHooks() ConversionHooks {
	return ConversionHooks{k}
}

// PostTxProcessing implements EvmHooks.PostTxProcessing. This handles minting
// sdk.Coin when ConvertToCoin() is called on an eligible Fury ERC20 contract.
func (h ConversionHooks) PostTxProcessing(
	ctx sdk.Context,
	msg core.Message,
	receipt *ethtypes.Receipt,
) error {
	erc20Abi := contract.ERC20MintableBurnableContract.ABI
	params := h.k.GetParams(ctx)

	for _, log := range receipt.Logs {
		// ERC20MintableBurnableContract ConvertToCoin event should contain 3 topics:
		// 0: Keccak-256 hash of ConvertToCoin(address,address,uint256)
		// 1: address indexed sender
		// 2: address indexed toAddr
		if len(log.Topics) != 3 {
			continue
		}

		// event ID, e.g. Keccak-256 hash of ConvertToCoin(address,address,uint256)
		eventID := log.Topics[0]

		event, err := erc20Abi.EventByID(eventID)
		if err != nil {
			// invalid event for ERC20
			continue
		}

		if event.Name != types.ContractEventTypeConvertToCoin {
			continue
		}

		convertToCoinEvent, err := erc20Abi.Unpack(event.Name, log.Data)
		if err != nil {
			h.k.Logger(ctx).Error("failed to unpack ConvertToCoin event", "error", err.Error())
			continue
		}

		if len(convertToCoinEvent) == 0 {
			h.k.Logger(ctx).Error("ConvertToCoin event data is empty", "error", err.Error())
			continue
		}

		// Data only contains non-indexed parameters, which is only the amount
		amount, ok := convertToCoinEvent[0].(*big.Int)
		// safety check and ignore if amount not positive
		if !ok || amount == nil || amount.Sign() != 1 {
			continue
		}

		// Check that the contract is enabled to convert to coin
		contractAddr := types.NewInternalEVMAddress(log.Address)
		conversionPair, err := h.k.GetEnabledConversionPairFromERC20Address(ctx, contractAddr)
		if err != nil {
			// Contract not a conversion pair in state
			continue
		}

		// Only check if bridge is disabled if the contract IS a conversion pair
		// in params to not affect user contracts. We want to return an error
		// instead of just skipping the conversion as we don't want users to be
		// able to send funds to the module account if the bridge is disabled ie
		// prevents loss of funds if account attempts to initiate a conversion
		// when bridge is disabled.
		if !params.BridgeEnabled {
			return types.ErrBridgeDisabled
		}

		initiator := common.BytesToAddress(log.Topics[1].Bytes())

		// Receiver is an sdk.AccAddress, but we use common.BytesToAddress
		// to remove the zero padding, then convert to AccAddress.
		receiverCommonAddr := common.BytesToAddress(log.Topics[2].Bytes())
		receiver := sdk.AccAddress(receiverCommonAddr.Bytes())

		// Does **not** check for Transfer event, assumes Contracts are trusted.

		// Initiator is a **different** address from receiver
		coin, err := h.k.MintConversionPairCoin(ctx, conversionPair, amount, receiver)
		if err != nil {
			// Revert tx if conversion fails
			panic(err)
		}

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeConvertERC20ToCoin,
			sdk.NewAttribute(types.AttributeKeyERC20Address, contractAddr.String()),
			sdk.NewAttribute(types.AttributeKeyInitiator, initiator.String()),
			sdk.NewAttribute(types.AttributeKeyReceiver, receiver.String()),
			sdk.NewAttribute(types.AttributeKeyAmount, coin.String()),
		))
	}

	return nil
}
