// +build evm

package gateway_v2

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	tgtypes "github.com/loomnetwork/go-loom/builtin/types/transfer_gateway"
	"github.com/loomnetwork/go-loom/client"
	ssha "github.com/miguelmota/go-solidity-sha3"
)

type MainnetGatewayClient struct {
	ethClient *ethclient.Client
	contract  *MainnetGatewayContract
	// Mainnet Gateway contract address
	Address   common.Address
	TxTimeout time.Duration
}

func (c *MainnetGatewayClient) Contract() *MainnetGatewayContract {
	return c.contract
}

func (c *MainnetGatewayClient) DepositERC20(caller *client.Identity, amount *big.Int, tokenAddr common.Address) error {
	tx, err := c.contract.DepositERC20(client.DefaultTransactOptsForIdentity(caller), amount, tokenAddr)
	if err != nil {
		return err
	}
	return client.WaitForTxConfirmation(context.TODO(), c.ethClient, tx, c.TxTimeout)
}

func (c *MainnetGatewayClient) DepositETH(caller *client.Identity, amount *big.Int) (*big.Int, error) {
	opts := client.DefaultTransactOptsForIdentity(caller)
	opts.Value = amount
	tx, err := c.contract.DepositEthToGateway(opts)
	if err != nil {
		return nil, err
	}
	return client.WaitForTxConfirmationAndFee(context.TODO(), c.ethClient, tx, c.TxTimeout)
}

func (c *MainnetGatewayClient) ERC721Deposited(tokenID *big.Int, tokenAddr common.Address) (bool, error) {
	return c.contract.GetERC721(nil, tokenID, tokenAddr)
}

func (c *MainnetGatewayClient) ERC721XBalance(tokenID *big.Int, tokenAddr common.Address) (*big.Int, error) {
	return c.contract.GetERC721X(nil, tokenID, tokenAddr)
}

func (c *MainnetGatewayClient) ERC20Balance(tokenAddr common.Address) (*big.Int, error) {
	return c.contract.GetERC20(nil, tokenAddr)
}

func (c *MainnetGatewayClient) Nonces(userAddr common.Address) (*big.Int, error) {
	return c.contract.Nonces(nil, userAddr)
}

func (c *MainnetGatewayClient) ETHBalance() (*big.Int, error) {
	return c.contract.GetETH(nil)
}

func (c *MainnetGatewayClient) WithdrawERC721(caller *client.Identity, tokenID *big.Int, tokenAddr common.Address, sigs []byte, validators []common.Address) error {
	hash := c.withdrawalHash(caller.MainnetAddr, tokenAddr, tgtypes.TransferGatewayTokenKind_ERC721, tokenID, big.NewInt(0))
	v, r, s, valIndexes, err := client.ParseSigs(sigs, hash, validators)
	if err != nil {
		return err
	}

	tx, err := c.contract.WithdrawERC721(client.DefaultTransactOptsForIdentity(caller), tokenID, tokenAddr, valIndexes, v, r, s)
	if err != nil {
		return err
	}
	return client.WaitForTxConfirmation(context.TODO(), c.ethClient, tx, c.TxTimeout)
}

func (c *MainnetGatewayClient) WithdrawERC721X(caller *client.Identity, tokenID, amount *big.Int, tokenAddr common.Address, sigs []byte, validators []common.Address) error {
	hash := c.withdrawalHash(caller.MainnetAddr, tokenAddr, tgtypes.TransferGatewayTokenKind_ERC721X, tokenID, amount)
	v, r, s, valIndexes, err := client.ParseSigs(sigs, hash, validators)
	if err != nil {
		return err
	}

	tx, err := c.contract.WithdrawERC721X(client.DefaultTransactOptsForIdentity(caller), tokenID, amount, tokenAddr, valIndexes, v, r, s)
	if err != nil {
		return err
	}
	return client.WaitForTxConfirmation(context.TODO(), c.ethClient, tx, c.TxTimeout)
}

func (c *MainnetGatewayClient) WithdrawERC20(caller *client.Identity, amount *big.Int, tokenAddr common.Address, sigs []byte, validators []common.Address) error {
	hash := c.withdrawalHash(caller.MainnetAddr, tokenAddr, tgtypes.TransferGatewayTokenKind_ERC20, big.NewInt(0), amount)
	v, r, s, valIndexes, err := client.ParseSigs(sigs, hash, validators)
	if err != nil {
		return err
	}

	tx, err := c.contract.WithdrawERC20(client.DefaultTransactOptsForIdentity(caller), amount, tokenAddr, valIndexes, v, r, s)
	if err != nil {
		return err
	}
	return client.WaitForTxConfirmation(context.TODO(), c.ethClient, tx, c.TxTimeout)
}

// WithdrawETH sends a tx to the Mainnet Gateway to withdraw the specified amount of ETH,
// and returns the tx fee.
func (c *MainnetGatewayClient) WithdrawETH(caller *client.Identity, amount *big.Int, sigs []byte, validators []common.Address) (*big.Int, error) {
	hash := c.withdrawalHash(caller.MainnetAddr, common.HexToAddress("0x0"), tgtypes.TransferGatewayTokenKind_ETH, big.NewInt(0), amount)
	v, r, s, valIndexes, err := client.ParseSigs(sigs, hash, validators)
	if err != nil {
		return nil, err
	}

	tx, err := c.contract.WithdrawETH(client.DefaultTransactOptsForIdentity(caller), amount, valIndexes, v, r, s)
	if err != nil {
		return nil, err
	}
	return client.WaitForTxConfirmationAndFee(context.TODO(), c.ethClient, tx, c.TxTimeout)
}

func (c *MainnetGatewayClient) withdrawalHash(withdrawer common.Address, tokenAddr common.Address, tokenKind tgtypes.TransferGatewayTokenKind, tokenId *big.Int, amount *big.Int) []byte {
	// Create hash of the message
	var hash []byte
	switch tokenKind {
	case tgtypes.TransferGatewayTokenKind_ERC721:
		hash = ssha.SoliditySHA3(
			[]string{"uint256", "address"},
			tokenId, tokenAddr,
		)
	case tgtypes.TransferGatewayTokenKind_ERC721X:
		hash = ssha.SoliditySHA3(
			[]string{"uint256", "uint256", "address"},
			tokenId, amount, tokenAddr,
		)
	case tgtypes.TransferGatewayTokenKind_ERC20:
		hash = ssha.SoliditySHA3(
			[]string{"uint256", "address"},
			amount, tokenAddr,
		)
	case tgtypes.TransferGatewayTokenKind_ETH:
		hash = ssha.SoliditySHA3(
			[]string{"uint256"},
			amount,
		)
	default:
		return nil
	}

	nonce, err := c.Nonces(withdrawer)
	if err != nil {
		return nil
	}

	// Make it non replayable
	hash = ssha.SoliditySHA3(
		[]string{"address", "uint256", "address", "bytes32"},
		withdrawer, nonce, c.Address, hash,
	)

	// Prefix the hash with the Ethereum Signed Message
	hash = ssha.SoliditySHA3(
		[]string{"string", "bytes32"},
		"\x19Ethereum Signed Message:\n32",
		hash,
	)

	return hash
}

func ConnectToMainnetGateway(ethClient *ethclient.Client, gatewayAddr string) (*MainnetGatewayClient, error) {
	contractAddr := common.HexToAddress(gatewayAddr)
	contract, err := NewMainnetGatewayContract(contractAddr, ethClient)
	if err != nil {
		return nil, err
	}
	return &MainnetGatewayClient{
		ethClient: ethClient,
		contract:  contract,
		Address:   contractAddr,
	}, nil
}
