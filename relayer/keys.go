package relayer

import (
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

const keyName = "sonic"

func NewSigner(cfg *Config) (keyring.Keyring, error) {
	k, err := GetKeyring(cfg)
	if err != nil {
		return nil, err
	}

	info, err := k.Key(keyName)
	if err != nil {
		return nil, err
	}
	if info != nil {
		// account already exists
		return k, nil
	}
	// we have to make an account. Generate one using the provided mnemonic
	keyringAlgos, _ := k.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString("secp256k1", keyringAlgos)
	if err != nil {
		return nil, err
	}

	var bip39Passphrase, hdPath string
	_, err = k.NewAccount(keyName, cfg.Mnemonic, bip39Passphrase, hdPath, algo)
	if err != nil {
		return nil, err
	}

	return k, nil
}

func GetKeyring(cfg *Config) (keyring.Keyring, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(homeDir, cfg.RootDir)
	kb, err := keyring.New(keyName, keyring.BackendTest, filePath, os.Stdin)
	if err != nil {
		return nil, err
	}
	return kb, nil
}
