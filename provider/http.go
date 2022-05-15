package provider

import (
	"context"
	"errors"
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
func (c *RPCClient) ValidatorSet(ctx context.Context, height *int64) (*tm.ValidatorSet, int64, error) {
	const maxPages = 100

	var (
		perPage = 100
		vals    = []*tm.Validator{}
		page    = 1
		total   = -1
	)

OUTER_LOOP:
	for attempts := 0; attempts < len(c.endpoints); attempts++ {
		if ctx.Err() != nil {
			return nil, -1, ctx.Err()
		}
		c.index = (c.index + 1) % len(c.endpoints)
		client, err := http.New(c.endpoints[c.index], "/websocket")
		if err != nil {
			log.Error().Err(err).Msg("creating rpc client")
			continue
		}
		log.Info().Str("address", c.endpoints[c.index]).Msg("created rpc client")
		for len(vals) != total && page <= maxPages {
			res, err := client.Validators(ctx, height, &page, &perPage)
			if err != nil {
				log.Error().Err(err)
				continue OUTER_LOOP
			}
			log.Info().Msg("received validator set")

			// check that the last height matches the one we requested
			if height != nil && *height != res.BlockHeight {
				continue OUTER_LOOP
			}
			if height == nil {
				// if we didn't check the height then update it so
				// we get validators for the correct height
				height = &res.BlockHeight
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
			return nil, -1, err
		}
		if err := valSet.ValidateBasic(); err != nil {
			return nil, -1, fmt.Errorf("returned validator set is invalid %v", err)
		}

		return valSet, *height, nil

	}
	if height == nil {
		return nil, -1, errors.New("no endpoint was able to provide the validator set at the latest height")
	}
	return nil, -1, fmt.Errorf("no endpoint was able to provide a valid validator set for height %d", height)
}
