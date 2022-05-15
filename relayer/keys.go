package relayer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/rs/zerolog/log"
)

const keyName = "sonic"

func NewSigner(cfg *Config) (keyring.Keyring, error) {
	k, err := GetKeyring(cfg)
	if err != nil {
		return nil, err
	}

	_, err = k.Key(keyName)
	if err == nil {
		log.Info().Msg("existing key found")
		// account already exists
		return k, nil
	}
	if err != nil && !strings.Contains(err.Error(), "key not found") {
		return nil, err
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
