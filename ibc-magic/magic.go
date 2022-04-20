package magic

import (
	b64 "encoding/base64"
)

func IBCMagic(txs []string) ([]byte, error) {
	decodedTx, err := b64.StdEncoding.DecodeString(txs[0])

	if err != nil {
		return nil, err
	}

	return decodedTx, err
}
