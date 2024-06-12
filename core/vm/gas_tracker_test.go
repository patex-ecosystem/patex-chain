package vm

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func setGasMode(state StateDB, contractAddress common.Address, mode *big.Int) {
	gasParameters := readGasParameters(state, contractAddress)
	gasParameters.mode = mode.Cmp(common.Big0) > 0
	updateGasParameters(state, contractAddress, gasParameters)
}

func getAddr(a uint64) common.Address {
	b := new(big.Int).SetUint64(a)
	return common.BigToAddress(b)
}

func checkGasTrackerStates(t *testing.T, gasTracker *GasTracker, gasUsed uint64, numAllocations uint64) {
	if gasTracker.GetGasUsed() != gasUsed {
		t.Error("Gas used not correct")
	}

	if len(gasTracker.allocations) != int(numAllocations) {
		t.Error("allocations should be made")
	}
}

func assertEtherBalance(t *testing.T, state StateDB, address common.Address, desiredBalance uint64) {
	gasParameters := readGasParameters(state, address)
	if address.String() == params.BlastGasAddress.String() {
		gasParameters.etherBalance = state.GetBalance(params.PatexBaseFeeRecipient)
	}
	if gasParameters.etherBalance.Cmp(new(big.Int).SetUint64(desiredBalance)) != 0 {
		t.Fatalf("ether balance incorrect, desired: %d, actual: %d", desiredBalance, gasParameters.etherBalance.Uint64())
	}
}

func assertLastUpdated(t *testing.T, state StateDB, address common.Address, desiredLastUpdated uint64) {
	gasParameters := readGasParameters(state, address)
	lastUpdated := gasParameters.lastUpdated
	if lastUpdated.Cmp(new(big.Int).SetUint64(desiredLastUpdated)) != 0 {
		t.Fatalf("last updated incorrect, desired: %d, actual: %d", desiredLastUpdated, lastUpdated.Uint64())
	}
}

func assertEtherSeconds(t *testing.T, state StateDB, address common.Address, desiredEtherSeconds uint64) {
	gasParameters := readGasParameters(state, address)
	etherSeconds := gasParameters.etherSeconds
	if etherSeconds.Cmp(new(big.Int).SetUint64(desiredEtherSeconds)) != 0 {
		t.Fatalf("ether seconds incorrect, desired: %d, actual: %d", desiredEtherSeconds, etherSeconds.Uint64())
	}
}

func TestGasTracker(t *testing.T) {
	gasTracker := NewGasTracker()

	if gasTracker == nil {
		t.Error("Failed to create new GasTracker")
	}
}

func TestInit(t *testing.T) {
	gasTracker := NewGasTracker()
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	timestamp := uint64(1)
	effectiveTip := big.NewInt(1)
	refund := uint64(0)

	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)
	checkGasTrackerStates(t, gasTracker, 0, 0)
}

func TestUnsetContract(t *testing.T) {
	gasTracker := NewGasTracker()

	gasTracker.UseGas(getAddr(1), 5)

	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	timestamp := uint64(1)
	effectiveTip := big.NewInt(1)
	refund := uint64(0)

	checkGasTrackerStates(t, gasTracker, 5, 1)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	blastEtherBalance := db.GetBalance(params.PatexBaseFeeRecipient)
	if blastEtherBalance.Cmp(new(big.Int).SetUint64(5)) != 0 {
		fmt.Println(blastEtherBalance)
		t.Fatalf("patex ether balance incorrect")
	}

	userBalance := readGasParameters(db, getAddr(1)).etherBalance
	if userBalance.Cmp(common.Big0) != 0 {
		t.Fatalf("user balance not correct")
	}
}

func TestMultipleUse(t *testing.T) {
	gasTracker := NewGasTracker()

	// use 5 gas
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(1), 5)

	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	timestamp := uint64(1)
	effectiveTip := big.NewInt(1)
	refund := uint64(9)

	checkGasTrackerStates(t, gasTracker, 10, 1)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	blastEtherBalance := db.GetBalance(params.PatexBaseFeeRecipient)
	if blastEtherBalance.Cmp(new(big.Int).SetUint64(1)) != 0 {
		t.Fatalf("patex ether balance incorrect")
	}

	userBalance := readGasParameters(db, getAddr(1)).etherBalance
	if userBalance.Cmp(common.Big0) != 0 {
		t.Fatalf("user balance not correct")
	}
}
func TestGasModeSet(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	setGasMode(db, getAddr(1), common.Big1)
	gasMode := readGasParameters(db, getAddr(1)).mode
	if gasMode == false {
		t.Fatalf("Gas mode is: %v", gasMode)
	}
}

func TestSetContract(t *testing.T) {

	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)

	// use gas when contract is set
	timestamp := uint64(1)
	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(1), 5)
	refund := uint64(9)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	// check gas mode
	setGasMode(db, getAddr(1), common.Big1)
	gasMode := readGasParameters(db, getAddr(1)).mode
	if gasMode == false {
		t.Fatalf("Gas mode is: %v", gasMode)
	}

	timestamp = uint64(2)
	gasTracker = NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(1), 5)
	refund = uint64(2)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	blastEtherBalance := db.GetBalance(params.PatexBaseFeeRecipient)
	if blastEtherBalance.Cmp(new(big.Int).SetUint64(1)) != 0 {
		fmt.Println(blastEtherBalance)
		t.Fatalf("patex ether balance incorrect")
	}

	userBalance := readGasParameters(db, getAddr(1)).etherBalance
	if userBalance.Cmp(new(big.Int).SetUint64(8)) != 0 {
		fmt.Println(userBalance)
		t.Fatalf("user balance not correct")
	}
}

func TestMultipleContracts(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)
	setGasMode(db, getAddr(1), common.Big1)
	setGasMode(db, getAddr(2), common.Big1)

	// use gas when contract is set
	timestamp := uint64(1)
	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(2), 5)
	gasTracker.UseGas(getAddr(3), 5)
	refund := uint64(0)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	assertEtherBalance(t, db, params.BlastGasAddress, 5)
	assertEtherBalance(t, db, getAddr(1), 5)
	assertEtherBalance(t, db, getAddr(2), 5)
	assertEtherBalance(t, db, getAddr(3), 0)
}

// TODO: -> fuzz these tests
func TestRefundContractWithoutBlastGas(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)
	setGasMode(db, getAddr(1), common.Big1)
	setGasMode(db, getAddr(2), common.Big1)

	// use gas when contract is set
	timestamp := uint64(1)
	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(2), 5)
	refund := uint64(1)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	assertEtherBalance(t, db, params.BlastGasAddress, 1)
	assertEtherBalance(t, db, getAddr(1), 4)
	assertEtherBalance(t, db, getAddr(2), 4)
}

func TestRefundContractWithBlastGas(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)
	setGasMode(db, getAddr(1), common.Big1)
	setGasMode(db, getAddr(2), common.Big1)

	// use gas when contract is set
	timestamp := uint64(1)
	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(2), 5)
	gasTracker.UseGas(getAddr(3), 5)
	refund := uint64(1)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, timestamp)

	assertEtherBalance(t, db, params.BlastGasAddress, 6)
	assertEtherBalance(t, db, getAddr(1), 4)
	assertEtherBalance(t, db, getAddr(2), 4)
	assertEtherBalance(t, db, getAddr(3), 0)
}

func TestEthBalanceAccumulation(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)
	setGasMode(db, getAddr(1), common.Big1)
	assertEtherBalance(t, db, getAddr(1), 0)

	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	refund := uint64(0)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 1)
	assertEtherBalance(t, db, getAddr(1), 5)

	gasTracker = NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 2)
	assertEtherBalance(t, db, getAddr(1), 10)
}

func TestLastUpdatedBase(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)
	setGasMode(db, getAddr(1), common.Big1)
	assertEtherBalance(t, db, getAddr(1), 0)
	assertLastUpdated(t, db, getAddr(1), 0)

	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	refund := uint64(0)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 1)
	assertLastUpdated(t, db, getAddr(1), 1)

}

func TestLastUpdatedUnset(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(1)
	setGasMode(db, getAddr(1), common.Big1)
	assertEtherBalance(t, db, getAddr(1), 0)
	assertLastUpdated(t, db, getAddr(1), 0)

	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	refund := uint64(0)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 1)
	assertLastUpdated(t, db, getAddr(1), 1)

	setGasMode(db, getAddr(1), common.Big0)
	gasTracker = NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 1)
	assertLastUpdated(t, db, getAddr(1), 1)

}

func TestGasBalanceInPredeploy(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	effectiveTip := big.NewInt(3)
	setGasMode(db, getAddr(1), common.Big1)
	setGasMode(db, getAddr(2), common.Big1)

	gasTracker := NewGasTracker()
	gasTracker.UseGas(getAddr(1), 5)
	gasTracker.UseGas(getAddr(2), 10)
	gasTracker.UseGas(getAddr(3), 10)
	refund := uint64(7)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 1)

	blastEtherBalance := db.GetBalance(params.PatexBaseFeeRecipient)
	if blastEtherBalance.Cmp(big.NewInt(24)) != 0 {
		t.Fatalf("Balance is not correct. Expected: %v, Got: %v", big.NewInt(24), blastEtherBalance)
	}
}

func TestEtherSecondsUpdate(t *testing.T) {
	db, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	setGasMode(db, getAddr(1), common.Big1)
	assertEtherBalance(t, db, getAddr(1), 0)
	assertLastUpdated(t, db, getAddr(1), 0)

	gasTracker := NewGasTracker()
	effectiveTip := big.NewInt(2)
	gasTracker.UseGas(getAddr(1), 5)
	refund := uint64(0)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 1)
	assertLastUpdated(t, db, getAddr(1), 1)
	assertEtherBalance(t, db, getAddr(1), 10)
	assertEtherSeconds(t, db, getAddr(1), 0)

	gasTracker = NewGasTracker()
	effectiveTip = big.NewInt(3)
	gasTracker.UseGas(getAddr(1), 2)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 2)
	assertEtherBalance(t, db, getAddr(1), 16)
	assertEtherSeconds(t, db, getAddr(1), 10)

	gasTracker = NewGasTracker()
	gasTracker.UseGas(getAddr(1), 20)
	gasTracker.AllocateDevGas(effectiveTip, refund, db, 10)
	assertEtherBalance(t, db, getAddr(1), 76)
	assertEtherSeconds(t, db, getAddr(1), 138)

}

func packAndUnpack(gasParams *GasParameters, t *testing.T) {
	packedBytes := pack(gasParams)
	if len(packedBytes) != 32 {
		t.Errorf("Packed bytes length is incorrect, got: %d, want: 32", len(packedBytes))
	}

	unpackedGasParams, err := unpack(packedBytes)
	if err != nil {
		t.Fatalf("Unpack failed with error: %v", err)
	}

	if unpackedGasParams.mode != gasParams.mode {
		t.Errorf("Unpacked mode is incorrect, got: %v, want: %v", unpackedGasParams.mode, gasParams.mode)
	}
	if unpackedGasParams.etherBalance.Cmp(gasParams.etherBalance) != 0 {
		t.Errorf("Unpacked etherBalance is incorrect, got: %v, want: %v", unpackedGasParams.etherBalance, gasParams.etherBalance)
	}
	if unpackedGasParams.etherSeconds.Cmp(gasParams.etherSeconds) != 0 {
		t.Errorf("Unpacked etherSeconds is incorrect, got: %v, want: %v", unpackedGasParams.etherSeconds, gasParams.etherSeconds)
	}
	if unpackedGasParams.lastUpdated.Cmp(gasParams.lastUpdated) != 0 {
		t.Errorf("Unpacked lastUpdated is incorrect, got: %v, want: %v", unpackedGasParams.lastUpdated, gasParams.lastUpdated)
	}
}

func TestPackUnpackGasParameters(t *testing.T) {
	gasParams := &GasParameters{
		mode:         true,
		etherBalance: big.NewInt(100),
		etherSeconds: big.NewInt(200),
		lastUpdated:  big.NewInt(300),
	}
	packAndUnpack(gasParams, t)
}

func TestZeroPackUnpack(t *testing.T) {
	// Test with mode false and zero balances
	gasParams := &GasParameters{
		mode:         false,
		etherBalance: big.NewInt(0),
		etherSeconds: big.NewInt(0),
		lastUpdated:  big.NewInt(0),
	}
	packAndUnpack(gasParams, t)
}

func TestMaxEtherBalance(t *testing.T) {
	// Test with mode true and extremely high balances
	oneEtherInWei, _ := new(big.Int).SetString("1000000000000000000", 10)
	maxEtherInCirculation, _ := new(big.Int).SetString("222700000", 10)
	maxWeiInCirculation := new(big.Int).Mul(oneEtherInWei, maxEtherInCirculation)
	gasParams := &GasParameters{
		mode:         true,
		etherBalance: maxWeiInCirculation,
		etherSeconds: big.NewInt(0),
		lastUpdated:  big.NewInt(0),
	}
	packAndUnpack(gasParams, t)
}

// 190 years for all ether in the world vested
func TestMaxEtherSeconds(t *testing.T) {
	// Test with mode true and extremely high balances
	oneEtherInWei, _ := new(big.Int).SetString("1000000000000000000", 10)
	maxEtherInCirculation, _ := new(big.Int).SetString("222700000", 10)
	maxWeiInCirculation := new(big.Int).Mul(oneEtherInWei, maxEtherInCirculation)
	for i := 1; i < 190; i++ {
		var years int64 = int64(i) * 365 * 24 * 60 * 60
		maxSeconds := new(big.Int).Mul(maxWeiInCirculation, big.NewInt(years))
		gasParams := &GasParameters{
			mode:         true,
			etherBalance: maxWeiInCirculation,
			etherSeconds: maxSeconds,
			lastUpdated:  big.NewInt(0),
		}
		fmt.Println(i)
		packAndUnpack(gasParams, t)

	}
}

// should panic
func TestInvalidBigInts(t *testing.T) {
	// Test with mode true and extremely high balances
	gasParams := &GasParameters{
		mode:         true,
		etherBalance: new(big.Int).Sub(big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1)),
		etherSeconds: new(big.Int).Sub(big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1)),
		lastUpdated:  new(big.Int).Sub(big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1)),
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	packAndUnpack(gasParams, t)
}

func TestMaxValues(t *testing.T) {
	gasParams := &GasParameters{
		mode:         true,
		etherBalance: getMaxValueForBits(12),
		etherSeconds: getMaxValueForBits(15),
		lastUpdated:  getMaxValueForBits(4),
	}
	packAndUnpack(gasParams, t)
}

func getMaxValueForBits(numberBits int64) *big.Int {
	return new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(numberBits), nil), common.Big1)
}
