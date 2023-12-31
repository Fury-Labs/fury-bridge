package types

// Events for the module
const (
	AttributeValueCategory = ModuleName

	// ERC20MintableBurnable event names
	ContractEventTypeWithdraw      = "Withdraw"
	ContractEventTypeConvertToCoin = "ConvertToCoin"

	// Event Types
	EventTypeBridgeEthereumToFury = "bridge_ethereum_to_fury"
	EventTypeBridgeFuryToEthereum = "bridge_fury_to_ethereum"
	EventTypeConvertERC20ToCoin   = "convert_erc20_to_coin"
	EventTypeConvertCoinToERC20   = "convert_coin_to_erc20"

	// Event Attributes - Common
	AttributeKeyReceiver = "receiver"
	AttributeKeyAmount   = "amount"

	// Event Attributes - Bridge
	AttributeKeyEthereumERC20Address = "ethereum_erc20_address"
	AttributeKeyFuryERC20Address     = "fury_erc20_address"
	AttributeKeyRelayer              = "relayer"
	AttributeKeySequence             = "sequence"

	// Event Attributes - Conversions
	AttributeKeyInitiator    = "initiator"
	AttributeKeyERC20Address = "erc20_address"
)
