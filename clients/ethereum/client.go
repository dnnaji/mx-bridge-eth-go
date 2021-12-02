package ethereum

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ElrondNetwork/elrond-eth-bridge/clients"
	"github.com/ElrondNetwork/elrond-eth-bridge/core"
	elrondCore "github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/core/check"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
)

const messagePrefix = "\u0019Ethereum Signed Message:\n32"

type argListsBatch struct {
	tokens     []common.Address
	recipients []common.Address
	amounts    []*big.Int
	nonces     []*big.Int
}

// ArgsEthereumClient is the DTO used in the ethereum's client constructor
type ArgsEthereumClient struct {
	ClientWrapper         ClientWrapper
	Erc20ContractsHandler Erc20ContractsHolder
	Log                   elrondCore.Logger
	AddressConverter      core.AddressConverter
	Broadcaster           Broadcaster
	PrivateKey            *ecdsa.PrivateKey
	TokensMapper          TokensMapper
	SignatureHolder       SignaturesHolder
	SafeContractAddress   common.Address
	GasHandler            GasHandler
	TransferGasLimit      uint64
}

type client struct {
	clientWrapper         ClientWrapper
	erc20ContractsHandler Erc20ContractsHolder
	log                   elrondCore.Logger
	addressConverter      core.AddressConverter
	broadcaster           Broadcaster
	privateKey            *ecdsa.PrivateKey
	publicKey             *ecdsa.PublicKey
	tokensMapper          TokensMapper
	signatureHolder       SignaturesHolder
	safeContractAddress   common.Address
	gasHandler            GasHandler
	transferGasLimit      uint64
}

// NewEthereumClient will create a new Ethereum client
func NewEthereumClient(args ArgsEthereumClient) (*client, error) {
	err := checkArgs(args)
	if err != nil {
		return nil, err
	}

	publicKey := args.PrivateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errPublicKeyCast
	}

	c := &client{
		clientWrapper:         args.ClientWrapper,
		erc20ContractsHandler: args.Erc20ContractsHandler,
		log:                   args.Log,
		addressConverter:      args.AddressConverter,
		broadcaster:           args.Broadcaster,
		privateKey:            args.PrivateKey,
		publicKey:             publicKeyECDSA,
		tokensMapper:          args.TokensMapper,
		signatureHolder:       args.SignatureHolder,
		safeContractAddress:   args.SafeContractAddress,
		gasHandler:            args.GasHandler,
		transferGasLimit:      args.TransferGasLimit,
	}

	c.log.Info("NewEthereumClient",
		"relayer address", crypto.PubkeyToAddress(*publicKeyECDSA),
		"safe contract address", c.safeContractAddress.String())

	return c, err
}

func checkArgs(args ArgsEthereumClient) error {
	if check.IfNil(args.ClientWrapper) {
		return errNilClientWrapper
	}
	if check.IfNil(args.Erc20ContractsHandler) {
		return errNilERC20ContractsHandler
	}
	if check.IfNil(args.Log) {
		return errNilLogger
	}
	if check.IfNil(args.AddressConverter) {
		return errNilAddressConverter
	}
	if check.IfNil(args.Broadcaster) {
		return errNilBroadcaster
	}
	if args.PrivateKey == nil {
		return errNilPrivateKey
	}
	if check.IfNil(args.TokensMapper) {
		return errNilTokensMapper
	}
	if check.IfNil(args.SignatureHolder) {
		return errNilSignaturesHolder
	}
	if check.IfNil(args.GasHandler) {
		return errNilGasHandler
	}
	if args.TransferGasLimit == 0 {
		return errInvalidGasLimit
	}

	return nil
}

// GetBatch returns the batch (if existing) from the Ethereum contract by providing the nonce
func (c *client) GetBatch(ctx context.Context, nonce uint64) *clients.TransferBatch {
	c.log.Info("Getting batch", "nonce", nonce)
	batch, err := c.clientWrapper.GetBatch(ctx, big.NewInt(0).SetUint64(nonce))
	if err != nil {
		// TODO maybe add filtering for the error here whenever the error will be defined
		c.log.Trace("getting batch", "error", err)

		return nil
	}

	transferBatch := &clients.TransferBatch{
		ID:       batch.Nonce.Uint64(),
		Deposits: make([]*clients.DepositTransfer, 0, len(batch.Deposits)),
	}

	for i := range batch.Deposits {
		deposit := batch.Deposits[i]
		toBytes := deposit.Recipient[:]
		fromBytes := deposit.Depositor[:]
		tokenBytes := deposit.TokenAddress[:]

		depositTransfer := &clients.DepositTransfer{
			Nonce:            deposit.Nonce.Uint64(),
			ToBytes:          toBytes,
			DisplayableTo:    c.addressConverter.ToBech32String(toBytes),
			FromBytes:        fromBytes,
			DisplayableFrom:  c.addressConverter.ToHexString(fromBytes),
			TokenBytes:       tokenBytes,
			DisplayableToken: c.addressConverter.ToHexString(tokenBytes),
			Amount:           big.NewInt(0).Set(deposit.Amount),
		}

		transferBatch.Deposits = append(transferBatch.Deposits, depositTransfer)
	}

	return transferBatch
}

// WasExecuted returns true if the batch ID was executed
func (c *client) WasExecuted(ctx context.Context, batchID uint64) (bool, error) {
	return c.clientWrapper.WasBatchExecuted(ctx, big.NewInt(0).SetUint64(batchID))
}

// BroadcastSignatureForMessageHash will send the signature for the provided message hash
func (c *client) BroadcastSignatureForMessageHash(msgHash common.Hash) {
	signature, err := crypto.Sign(msgHash.Bytes(), c.privateKey)
	if err != nil {
		c.log.Error("error generating signature", "msh hash", msgHash, "error", err)
		return
	}

	c.broadcaster.BroadcastSignature(signature, msgHash.Bytes())
}

// GenerateMessageHash will generate the message hash based on the provided batch
func (c *client) GenerateMessageHash(ctx context.Context, batch *clients.TransferBatch) (common.Hash, error) {
	if batch == nil {
		return common.Hash{}, errNilBatch
	}

	args, err := generateTransferArgs()
	if err != nil {
		return common.Hash{}, err
	}

	argLists, err := c.extractList(ctx, batch)
	if err != nil {
		return common.Hash{}, err
	}

	pack, err := args.Pack(argLists.tokens, argLists.recipients, argLists.amounts, argLists.nonces, big.NewInt(0).SetUint64(batch.ID), "ExecuteBatchedTransfer")
	if err != nil {
		return common.Hash{}, err
	}

	hash := crypto.Keccak256Hash(pack)
	return crypto.Keccak256Hash(append([]byte(messagePrefix), hash.Bytes()...)), nil
}

func generateTransferArgs() (abi.Arguments, error) {
	addressesType, err := abi.NewType("address[]", "", nil)
	if err != nil {
		return nil, err
	}

	uint256ArrayType, err := abi.NewType("uint256[]", "", nil)
	if err != nil {
		return nil, err
	}

	uint256Type, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return nil, err
	}

	stringType, err := abi.NewType("string", "", nil)
	if err != nil {
		return nil, err
	}

	return abi.Arguments{
		abi.Argument{Name: "tokens", Type: addressesType},
		abi.Argument{Name: "recipients", Type: addressesType},
		abi.Argument{Name: "amounts", Type: uint256ArrayType},
		abi.Argument{Name: "nonces", Type: uint256ArrayType},
		abi.Argument{Name: "nonce", Type: uint256Type},
		abi.Argument{Name: "executeTransfer", Type: stringType},
	}, nil
}

func (c *client) extractList(ctx context.Context, batch *clients.TransferBatch) (argListsBatch, error) {
	arg := argListsBatch{}

	for _, dt := range batch.Deposits {
		recipient := common.BytesToAddress(dt.ToBytes)
		arg.recipients = append(arg.recipients, recipient)

		erc20TokenBytes, err := c.tokensMapper.ConvertToken(ctx, dt.TokenBytes)
		if err != nil {
			return argListsBatch{}, err
		}

		token := common.BytesToAddress(erc20TokenBytes)
		arg.tokens = append(arg.tokens, token)

		amount := big.NewInt(0).Set(dt.Amount)
		arg.amounts = append(arg.amounts, amount)

		nonce := big.NewInt(0).SetUint64(dt.Nonce)
		arg.nonces = append(arg.nonces, nonce)
	}

	return arg, nil
}

// ExecuteTransfer will initiate and send the transaction from the transfer batch struct
func (c *client) ExecuteTransfer(
	ctx context.Context,
	msgHash common.Hash,
	batch *clients.TransferBatch,
	quorum int,
) (string, error) {
	if batch == nil {
		return "", errNilBatch
	}

	c.log.Info("executing transfer " + batch.String())

	fromAddress := crypto.PubkeyToAddress(*c.publicKey)

	nonce, err := c.getNonce(ctx, fromAddress)
	if err != nil {
		return "", err
	}

	chainId, err := c.clientWrapper.ChainID(ctx)
	if err != nil {
		return "", err
	}

	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, chainId)
	if err != nil {
		return "", err
	}

	gasPrice, err := c.gasHandler.GetCurrentGasPrice()
	if err != nil {
		return "", err
	}

	auth.Nonce = big.NewInt(nonce)
	auth.Value = big.NewInt(0)
	auth.GasLimit = c.transferGasLimit
	auth.Context = ctx
	auth.GasPrice = gasPrice

	signatures := c.signatureHolder.Signatures(msgHash.Bytes())
	if len(signatures) < quorum {
		return "", fmt.Errorf("%w num signatures: %d, quorum: %d", errQuorumNotReached, len(signatures), quorum)
	}
	if len(signatures) > quorum {
		c.log.Debug("reducing the size of the signatures set",
			"quorum", quorum, "total signatures", len(signatures))
		signatures = signatures[:quorum]
	}

	argLists, err := c.extractList(ctx, batch)
	if err != nil {
		return "", err
	}

	err = c.checkAvailableTokens(ctx, argLists.tokens, argLists.amounts)
	if err != nil {
		return "", err
	}

	batchID := big.NewInt(0).SetUint64(batch.ID)
	tx, err := c.clientWrapper.ExecuteTransfer(auth, argLists.tokens, argLists.recipients, argLists.amounts, argLists.nonces, batchID, signatures)
	if err != nil {
		return "", err
	}

	txHash := tx.Hash().String()
	c.log.Info("Executed transfer transaction", "batchID", batchID, "hash", txHash)

	return txHash, err
}

func (c *client) checkAvailableTokens(ctx context.Context, tokens []common.Address, amounts []*big.Int) error {
	transfers := c.getCumulatedTransfers(tokens, amounts)

	return c.checkCumulatedTransfers(ctx, transfers)
}

func (c *client) getCumulatedTransfers(tokens []common.Address, amounts []*big.Int) map[common.Address]*big.Int {
	transfers := make(map[common.Address]*big.Int)
	for i, token := range tokens {
		existing, found := transfers[token]
		if !found {
			existing = big.NewInt(0)
			transfers[token] = existing
		}

		existing.Add(existing, amounts[i])
	}

	return transfers
}

func (c *client) checkCumulatedTransfers(ctx context.Context, transfers map[common.Address]*big.Int) error {
	for erc20Address, value := range transfers {
		existingBalance, err := c.erc20ContractsHandler.BalanceOf(ctx, erc20Address, c.safeContractAddress)
		if err != nil {
			return fmt.Errorf("%w for address %s for ERC20 token %s", err, c.safeContractAddress.String(), erc20Address.String())
		}

		if value.Cmp(existingBalance) > 0 {
			return fmt.Errorf("%w, existing: %s, required: %s for ERC20 token %s and address %s",
				errInsufficientErc20Balance, existingBalance.String(), value.String(), erc20Address.String(), c.safeContractAddress.String())
		}

		c.log.Debug("checked ERC20 balance",
			"ERC20 token", erc20Address.String(),
			"address", c.safeContractAddress.String(),
			"existing balance", existingBalance.String(),
			"needed", value.String())
	}

	return nil
}

func (c *client) getNonce(ctx context.Context, fromAddress common.Address) (int64, error) {
	blockNonce, err := c.clientWrapper.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w in getNonce, BlockNumber call", err)
	}

	nonce, err := c.clientWrapper.NonceAt(ctx, fromAddress, big.NewInt(int64(blockNonce)))

	return int64(nonce), err
}

// IsInterfaceNil returns true if there is no value under the interface
func (c *client) IsInterfaceNil() bool {
	return c == nil
}