package ibc_test

import (
	"testing"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/plural-labs/sonic-relayer/ibc"
)

func TestPrepareAndSign(t *testing.T) {
	signer := keyring.NewInMemory()

	info, _, err := signer.NewMnemonic("test", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)

	testChains := map[string]*ibc.Account{
		"test-chain-1": {
			Prefix:        "cosmos",
			AccountNumber: 3,
			Sequence:      2,
		},
		"test-chain-2": {
			Prefix:        "terra",
			AccountNumber: 18,
			Sequence:      5,
		},
	}

	accountant, err := ibc.NewAccountant(signer, info.GetAddress(), testChains)
	require.NoError(t, err)

	require.Equal(t, info.GetAddress(), accountant.Address())

	addr2 := secp256k1.GenPrivKey().PubKey().Address()

	transferMsg := bank.NewMsgSend(accountant.Address(), sdk.AccAddress(addr2), sdk.NewCoins(sdk.NewInt64Coin("stake", 1000)))

	// happy path
	tx, err := accountant.PrepareAndSign([]sdk.Msg{transferMsg}, "test-chain-1")
	require.NoError(t, err)
	require.NoError(t, tx.ValidateBasic())
	require.Equal(t, []sdk.Msg{transferMsg}, tx.GetMsgs())

	// errors with no message
	_, err = accountant.PrepareAndSign([]sdk.Msg{}, "test-chain-2")
	require.Error(t, err)

	// errors with wrong chain-id
	_, err = accountant.PrepareAndSign([]sdk.Msg{transferMsg}, "test-chain-3")
	require.Error(t, err)

	// errors with wrong signer
	transferMsg2 := bank.NewMsgSend(sdk.AccAddress(addr2), accountant.Address(), sdk.NewCoins(sdk.NewInt64Coin("stake", 100)))
	_, err = accountant.PrepareAndSign([]sdk.Msg{transferMsg2}, "test-chain-2")
	require.Error(t, err)
}

func mockAccountant(chainId string) *ibc.Accountant {
	signer := keyring.NewInMemory()

	info, _, err := signer.NewMnemonic("test", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	if err != nil {
		panic(err)
	}
	accountant, err := ibc.NewAccountant(signer, info.GetAddress(), map[string]*ibc.Account{
		chainId: {
			Prefix:        "cosmos",
			AccountNumber: 1,
			Sequence:      1,
		},
	})
	if err != nil {
		panic(err)
	}
	return accountant
}
