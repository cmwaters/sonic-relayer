package ibc_test

import (
	"github.com/stretchr/testify/suite"

	handler "github.com/plural-labs/sonic-relayer/ibc"
	mocks "github.com/plural-labs/sonic-relayer/testing/mocks"
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

			tc.malleate()
			ibcHandler := handler.NewHandler()
			err := ibcHandler.Process(mockTxs)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
