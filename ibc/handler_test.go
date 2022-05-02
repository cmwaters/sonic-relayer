package ibc_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/plural-labs/sonic-relayer/ibc"
	mocks "github.com/plural-labs/sonic-relayer/testing/mocks"
	"github.com/plural-labs/sonic-relayer/tx"
)

type HandlerTestSuite struct {
	suite.Suite
}

func (suite *HandlerTestSuite) TestIBCHandler() {
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
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			mockTxs := mocks.BuildMockBlock()
			sourceEndpoint := ibc.Endpoint{}
			destEndpoint := ibc.Endpoint{}

			tc.malleate()
			counterpartyMempool := tx.NewMempool()
			ibcHandler := ibc.NewHandler(counterpartyMempool, mockAccountant("test-chain"), sourceEndpoint, destEndpoint)
			err := ibcHandler.Process(mockTxs)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
