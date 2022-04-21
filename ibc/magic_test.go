package ibc_test

import (
	"github.com/stretchr/testify/suite"

	magic "github.com/plural/sonic-relayer/ibc"
	mocks "github.com/plural/sonic-relayer/testing/mocks"
)

type MagicTestSuite struct {
	suite.Suite
}

func (suite *MagicTestSuite) TestIBCMagic() {
	var (
		mockTxs []string
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success",
			func() {},
			true,
		},
		{
			"error decoding txs",
			func() {
				mockTxs = []string{"1"}
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			mockTxs = mocks.Base64EncodedTxs

			tc.malleate()
			_, err := magic.IBCMagic(mockTxs)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
