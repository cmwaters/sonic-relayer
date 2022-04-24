package provider

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/rpc/client/http"
	tm "github.com/tendermint/tendermint/types"
)

type RPCClient struct {
	index     int
	endpoints []string
}

func NewRPCClient(endpoints []string) *RPCClient {
	return &RPCClient{
		endpoints: endpoints,
		index:     0,
	}
}

// ValidatorSet queries the available endpoints for the validator set of the chain. It only errors
// if none of the endpoints return a valid validator set or a context is cancelled.
func (c *RPCClient) ValidatorSet(ctx context.Context, height *int64) (*tm.ValidatorSet, error) {
	const maxPages = 100

	var (
		perPage       = 100
		vals          = []*tm.Validator{}
		page          = 1
		total         = -1
		startingIndex = c.index
	)

OUTER_LOOP:
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c.index = (c.index + 1) % len(c.endpoints)
		if c.index == startingIndex {
			return nil, fmt.Errorf("no endpoint was able to provide a valid validator set for height %d", height)
		}
		client, err := http.New(c.endpoints[c.index], "/websocket")
		if err != nil {
			log.Error().Err(err)
			continue
		}
		for len(vals) != total && page <= maxPages {
			res, err := client.Validators(ctx, height, &page, &perPage)
			if err != nil {
				log.Error().Err(err)
				continue OUTER_LOOP
			}

			// Validate response.
			if len(res.Validators) == 0 {
				log.Error().Err(fmt.Errorf("validator set is empty (height: %d, page: %d, per_page: %d)",
					height, page, perPage))
				continue OUTER_LOOP
			}
			if res.Total <= 0 {
				log.Error().Err(fmt.Errorf("total number of vals is <= 0: %d (height: %d, page: %d, per_page: %d)",
					res.Total, height, page, perPage))
				continue OUTER_LOOP
			}

			total = res.Total
			vals = append(vals, res.Validators...)
			page++
		}

		valSet, err := tm.ValidatorSetFromExistingValidators(vals)
		if err != nil {
			return nil, err
		}
		if err := valSet.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("returned validator set is invalid %v", err)
		}

		return valSet, nil

	}
}
