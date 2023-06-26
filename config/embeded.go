package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
)

func mustParseGenesisConfigFromJson(rawJson []byte) *core.Genesis {
	genesis := new(core.Genesis)
	if err := json.Unmarshal(rawJson, genesis); err != nil {
		panic(fmt.Sprintf("invalid genesis file: %v", err))
	}
	return genesis
}

//go:embed embedded/genesis-sepolia.json
var sepoliaRawGenesisConfig []byte

var PatexSepoliaGenesisConfig = mustParseGenesisConfigFromJson(sepoliaRawGenesisConfig)

//go:embed embedded/genesis-mainnet.json
var mainnetRawGenesisConfig []byte

var PatexMainnetGenesisConfig = mustParseGenesisConfigFromJson(mainnetRawGenesisConfig)
