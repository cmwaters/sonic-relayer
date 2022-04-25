package provider

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/bytes"
	tm "github.com/tendermint/tendermint/types"
	"google.golang.org/grpc"
)

type GRPCClient struct {
	address string
}

func NewGRPCClient(address string) *GRPCClient {
	return &GRPCClient{
		address: address,
	}
}

func (p GRPCClient) ValidatorSet(ctx context.Context, height int64) (*tm.ValidatorSet, error) {
	conn, err := grpc.Dial(p.address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	tmServer := tmservice.NewServiceClient(conn)
	resp, err := tmServer.GetValidatorSetByHeight(ctx, &tmservice.GetValidatorSetByHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}

	valz := make([]*tm.Validator, len(resp.Validators))
	for idx, validator := range resp.Validators {
		var pk cryptotypes.PubKey
		err := proto.Unmarshal(validator.PubKey.Value, pk)
		if err != nil {
			return nil, err
		}

		valz[idx] = &tm.Validator{
			Address:          bytes.HexBytes(validator.Address),
			VotingPower:      validator.VotingPower,
			ProposerPriority: validator.ProposerPriority,
			// FIXME: wtf.. SDK uses it's own public key interface which is different to
			// tendermint's which so far I can't find a way to convert it to the tendermint
			// public key that we need
			// PubKey: pk,
		}
	}
	return tm.NewValidatorSet(valz), nil
}
