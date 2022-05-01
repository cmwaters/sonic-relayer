package ibc

import (
	"errors"
	"fmt"

	codec "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

type Accountant struct {
	signer        keyring.Keyring
	address       sdk.AccAddress
	chainAccounts map[string]*Account // chain_id to prefix
}

type Account struct {
	Prefix        string
	Sequence      uint64
	AccountNumber uint64
}

func NewAccountant(signer keyring.Keyring, address sdk.AccAddress, accounts map[string]*Account) (*Accountant, error) {
	_, err := signer.KeyByAddress(address)
	if err != nil {
		return nil, err
	}
	return &Accountant{
		signer:        signer,
		address:       address,
		chainAccounts: accounts,
	}, nil
}

// PrepareAndSign takes an array of messages, creates a
// transaction from them, adding meta data like gas and
// then signing the transaction, returning the completed tx.
func (a *Accountant) PrepareAndSign(msgs []sdk.Msg, chainID string) (tx.Tx, error) {
	if len(msgs) == 0 {
		return tx.Tx{}, errors.New("must contain at least one message")
	}

	anyMsgs := make([]*codec.Any, len(msgs))
	for idx, msg := range msgs {
		err := msg.ValidateBasic()
		if err != nil {
			return tx.Tx{}, fmt.Errorf("invalid msg (idx: %d): %w", idx, err)
		}

		anyMsgs[idx], err = codec.NewAnyWithValue(msg)
		if err != nil {
			return tx.Tx{}, fmt.Errorf("marshalling msg (idx: %d): %w", idx, err)
		}
	}

	account, ok := a.chainAccounts[chainID]
	if !ok {
		return tx.Tx{}, fmt.Errorf("no recorded account exists for chain %s", chainID)
	}

	Tx := tx.Tx{
		Body: &tx.TxBody{Messages: anyMsgs},
		AuthInfo: &tx.AuthInfo{
			SignerInfos: []*tx.SignerInfo{
				{
					ModeInfo: &tx.ModeInfo{
						Sum: &tx.ModeInfo_Single_{
							Single: &tx.ModeInfo_Single{Mode: signing.SignMode_SIGN_MODE_DIRECT},
						},
					},
					Sequence: account.Sequence,
				},
			},
			Fee: &tx.Fee{
				// FIXME: This is an arbitrary value. We should have a better way of estimating gas for our transactions
				GasLimit: 2000000,
			},
		},
	}

	bodyBytes, err := Tx.Body.Marshal()
	if err != nil {
		return tx.Tx{}, err
	}
	authInfoBytes, err := Tx.AuthInfo.Marshal()
	if err != nil {
		return tx.Tx{}, err
	}

	signDoc := &tx.SignDoc{
		BodyBytes:     bodyBytes,
		AuthInfoBytes: authInfoBytes,
		ChainId:       chainID,
		AccountNumber: account.AccountNumber,
	}
	signedBytes, err := signDoc.Marshal()
	if err != nil {
		return tx.Tx{}, err
	}

	signers := Tx.GetSigners()
	if len(signers) > 1 {
		return tx.Tx{}, fmt.Errorf("expected a single signer, got %d", len(signers))
	}

	sig, _, err := a.signer.SignByAddress(signers[0], signedBytes)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("failed to sign message: %w", err)
	}
	Tx.Signatures = [][]byte{sig}

	// increment account number
	account.AccountNumber++
	return Tx, nil
}

// HumanAddress returns a bech32 encoded address (including
// chain prefix and checksum) as a string. Can be used
// as the signer in messages.
func (a Accountant) HumanAddress(chainID string) (string, error) {
	acc, ok := a.chainAccounts[chainID]
	if !ok {
		return "", fmt.Errorf("no address matching to the chain ID %s exists", chainID)
	}
	bech32Address, err := bech32.ConvertAndEncode(acc.Prefix, a.address)
	if err != nil {
		panic(err)
	}

	return bech32Address, nil
}

// Address returns the generic hex encoded address derived
// from the accounts public key.
func (a Accountant) Address() sdk.AccAddress {
	return a.address
}
