package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

type GasParameters struct {
	mode         bool
	lastUpdated  *big.Int
	etherBalance *big.Int
	etherSeconds *big.Int
}

type GasTracker struct {
	allocations map[common.Address]uint64
	gasUsed     uint64
}

func (gtm *GasTracker) GetGasUsedByContract(address common.Address) uint64 {
	return gtm.allocations[address]
}

func (gtm *GasTracker) GetGasUsed() uint64 {
	return gtm.gasUsed
}

func (gtm *GasTracker) UseGas(address common.Address, amount uint64) {
	gtm.gasUsed += amount
	gtm.allocations[address] += amount
}

func (gtm *GasTracker) RefundGas(address common.Address, amount uint64) {
	// sanity check
	if amount > gtm.gasUsed || amount > gtm.allocations[address] {
		panic(fmt.Sprintf("refund exceeds gas used: gasUsed=%d, allocation=%d, amount=%d", gtm.gasUsed, gtm.allocations[address], amount))
	}

	gtm.gasUsed -= amount
	gtm.allocations[address] -= amount
}

func (gtm *GasTracker) AllocateDevGas(gasPrice *big.Int, refund uint64, state StateDB, timestamp uint64) {
	// net gas used is 0 or gas consumed is <= refund
	if gtm.gasUsed == 0 || gtm.gasUsed <= refund {
		return
	}

	remainingGas := new(big.Int).SetUint64(gtm.gasUsed - refund)
	netGas := new(big.Int).SetUint64(gtm.gasUsed)
	accumulatedGas := new(big.Int)
	totalGasAccount := new(big.Int)
	blockTimestamp := new(big.Int).SetUint64(timestamp)
	for addr, rawAmount := range gtm.allocations {
		// find scaled gas units
		parsedRawAmount := new(big.Int).SetUint64(rawAmount)
		scaledGasUnits := new(big.Int).Div(new(big.Int).Mul(remainingGas, parsedRawAmount), netGas)
		totalGasAccount.Add(totalGasAccount, scaledGasUnits)

		// skip allocation of gas to contracts that dont accumulate
		gasParameters := readGasParameters(state, addr)
		if !gasParameters.mode {
			continue
		}

		accumulatedGas.Add(accumulatedGas, scaledGasUnits)

		// calculate gas in wei terms
		fee := new(big.Int).Mul(scaledGasUnits, gasPrice)

		// update gas predeploy
		if fee.Cmp(common.Big0) > 0 {
			updateGasPredeploy(state, addr, fee, blockTimestamp, gasParameters)
		}
	}

	// sanity check
	if totalGasAccount.Cmp(remainingGas) > 0 {
		panic(fmt.Sprintf("gas accounting inflation: totalGasAccount=%v, remainingGas=%v", totalGasAccount.String(), remainingGas.String()))
	}

	// give rest of gas to base fee recipient (for patex admin to claim)
	patexGasUnits := new(big.Int).Sub(remainingGas, accumulatedGas)
	patexGas := new(big.Int).Mul(patexGasUnits, gasPrice)
	state.AddBalance(params.PatexBaseFeeRecipient, patexGas)

	// pay out non-void gas to patex predeploy
	claimableGasToAdd := new(big.Int).Mul(accumulatedGas, gasPrice)
	state.AddBalance(params.PatexGasAddress, claimableGasToAdd)
}

func NewGasTracker() *GasTracker {
	return &GasTracker{
		allocations: make(map[common.Address]uint64),
		gasUsed:     0,
	}
}

func updateGasPredeploy(state StateDB, contractAddress common.Address, fee *big.Int, timestamp *big.Int, gasParameters *GasParameters) {
	unprocessedEtherSeconds := new(big.Int).Mul(gasParameters.etherBalance, new(big.Int).Sub(timestamp, gasParameters.lastUpdated))
	gasParameters.etherSeconds = new(big.Int).Add(gasParameters.etherSeconds, unprocessedEtherSeconds)
	gasParameters.etherBalance = new(big.Int).Add(gasParameters.etherBalance, fee)
	gasParameters.lastUpdated = timestamp
	updateGasParameters(state, contractAddress, gasParameters)
}

func readGasParameters(state StateDB, contractAddress common.Address) *GasParameters {
	slot := getContractStorageSlot(contractAddress)
	gasStorageSlotBytes := state.GetState(params.PatexGasAddress, slot).Bytes()
	gasParameters, err := unpack(gasStorageSlotBytes)
	if err != nil {
		// TODO(patex): remove this panic, should never happen
		panic(err)
	}
	return gasParameters
}

func updateGasParameters(state StateDB, contractAddress common.Address, gasParameters *GasParameters) {
	slot := getContractStorageSlot(contractAddress)
	byteArray := pack(gasParameters)
	state.SetState(params.PatexGasAddress, slot, common.BytesToHash(byteArray))
}

func unpack(params []byte) (*GasParameters, error) {
	if len(params) != 32 {
		return nil, errors.New("storage slot must contain 32 bytes")
	}
	solidityInput := newSolidityInput(params)
	rawGasMode, err := solidityInput.consumeBytes(1)
	if err != nil {
		return nil, err
	}
	gasMode := rawGasMode[0] != 0
	rawEtherBytes, err := solidityInput.consumeBytes(12)
	if err != nil {
		return nil, err
	}
	etherBalance := new(big.Int).SetBytes(rawEtherBytes)

	rawEtherSecondsBytes, err := solidityInput.consumeBytes(15)
	if err != nil {
		return nil, err
	}
	etherSeconds := new(big.Int).SetBytes(rawEtherSecondsBytes)

	rawLastUpdatedBytes, err := solidityInput.consumeBytes(4)
	if err != nil {
		return nil, err
	}
	lastUpdated := new(big.Int).SetBytes(rawLastUpdatedBytes)

	gasParameters := &GasParameters{
		mode:         gasMode,
		etherBalance: etherBalance,
		etherSeconds: etherSeconds,
		lastUpdated:  lastUpdated,
	}
	return gasParameters, nil
}

func pack(params *GasParameters) []byte {
	output := make([]byte, 32)

	if params.mode {
		output[0] = 1
	}

	// This will panic if any of the values exceeds the buffer size
	// See: https: //pkg.go.dev/math/big#Int.FillBytes
	params.etherBalance.FillBytes(output[1:13])
	params.etherSeconds.FillBytes(output[13:28])
	params.lastUpdated.FillBytes(output[28:32])
	return output
}

func getContractStorageSlot(contractAddress common.Address) common.Hash {
	slot := getHash(contractAddress, "parameters")
	return slot
}

func getHash(addr common.Address, app string) common.Hash {
	strBytes := []byte(app)
	output := append(addr.Bytes(), strBytes...)
	hash := crypto.Keccak256Hash(output)
	return hash
}
