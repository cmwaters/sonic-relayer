package mocks

import (
	"encoding/json"

	tm "github.com/tendermint/tendermint/types"
)

// TODO: hackyaf but just pulled these from osmosis for quick feedback. We can create some helpers to build useful txs next
var txBytes = []byte(
	`
data: {
  txs: 
    [
     "Co8BCowBChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEmwKLWNvc21vczFubjg1cndqeWtlbTByZG1tZW1mbTZyeTNkZWVxOG14a2t4ZjRzdRItY29zbW9zMW5uODVyd2p5a2VtMHJkbW1lbWZtNnJ5M2RlZXE4bXhra3hmNHN1GgwKBXVhdG9tEgMxNTASZgpQCkYKHy9jb3Ntb3MuY3J5cHRvLnNlY3AyNTZrMS5QdWJLZXkSIwohA6P7GHglhvJOQkvwOWroD5kZg5nNiT/ULcOB9JqCgEUZEgQKAggBGBMSEgoMCgV1YXRvbRIDMTUxEPCTCRpAsHxzr06wV9GBOvbsJkar/lXvwcEIMWr5XUZvye4UB7F7Sq7Ses66IpReb277jwd+UaR3OeccVADOvM/ARjdPKg==",
	   "cqmbcp4bcimvy29zbw9zlnn0ywtpbmcudjfizxrhms5nc2dezwxlz2f0zrj3ci1jb3ntb3mxzmq1ownkc3p5oghsemz1m2rjexvuchrzntl3ewd2m2ponxz4a2esngnvc21vc3zhbg9wzxixbhpobg5wywh2em53zny0am1hetj0z2foytvrbxo1cxhlcmfycmwaeaofdwf0b20sbzewmdawmdasabjnclakrgofl2nvc21vcy5jcnlwdg8uc2vjcdi1nmsxllb1yktlerijciecnpl5cuoc/4gnz7k4elydbdomubnkrltsmkyn7ofkj6wsbaocch8ychitcg0kbxvhdg9tegqyntawejchdxpa2z5xqld2cnpdon5fr4gjlu2sicliuhkyuzqysyv3bllojz4opdpff1n8souk9omq8scxsntsi0pfd/dgt3grjg==",
	   "CpIBCo8BChwvY29zbW9zLmJhbmsudjFiZXRhMS5Nc2dTZW5kEm8KLWNvc21vczFoZm1sNHR6d2xjM212eW5zZzZ2dGd5d3l4MDB3ZmtocnRwa3g2dBItY29zbW9zMWg3eTk3cGR2cHQyODhwN2Vxanpqdm00dTJkOWZndHNkNGpnaDV5Gg8KBXVhdG9tEgYxMDAxMDASaQpSCkYKHy9jb3Ntb3MuY3J5cHRvLnNlY3AyNTZrMS5QdWJLZXkSIwohAqZQBjdpGqfv6znFTLFfHpqM91IfJ4I77xOMNJvSvOsSEgQKAggBGNC1BxITCg0KBXVhdG9tEgQ1MDAwEMCaDBpABHLVjMkVvkZe9gFdSPldgb/725+seyGLcyqsoQ+P9vhrsU60NxXt79sqz/ix21LW1Zej2y5QcbD2PjSUPYgx5Q==",
    ]
  }
`,
)

func BuildMockBlock() *tm.Block {
	var block tm.Block

	if err := json.Unmarshal(txBytes, block.Data); err != nil {
		panic(err)
	}

	return &block
}
