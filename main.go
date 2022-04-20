package main

import (
	"fmt"

	magic "github.com/plural/sonic-relayer/ibc-magic"
	mocks "github.com/plural/sonic-relayer/testing/mocks"
)

func main() {
	mockTxs := mocks.Base64EncodedTxs
	decodedTx, _ := magic.IBCMagic(mockTxs)
	fmt.Println(decodedTx)
}
