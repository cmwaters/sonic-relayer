# Sonic Relayer

Sonic The Relayer. 

![](https://deluxe.news/wp-content/uploads/2022/04/1650461660_Sonic-Origins-is-the-remastered-classic-Sonic-collection-coming-June-780x470.png)

# simd scripts

Boilerplate scripts for boostrapping simd nodes and relayer

### Installing simd

Install the `simd` binary from the [`ibc-go`](https://github.com/cosmos/ibc-go) repository.

```bash
git clone git@github.com:cosmos/ibc-go.git && cd ibc-go

make install

make start-rly

// Create Channel
hermes -c ./network/hermes/config.toml create channel test-1 --connection-a connection-0 --port-a transfer --port-b transfer -v "ics20-1"

// Query Channel
simd q ibc channel channels --home ./data/test-1 --node tcp://localhost:16657
```

