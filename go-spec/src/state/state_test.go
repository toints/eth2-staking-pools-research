package state

import (
	"encoding/hex"
	"github.com/bloxapp/eth2-staking-pools-research/go-spec/src/block"
	"github.com/bloxapp/eth2-staking-pools-research/go-spec/src/core"
	"github.com/bloxapp/eth2-staking-pools-research/go-spec/src/mocks"
	"github.com/bloxapp/eth2-staking-pools-research/go-spec/src/shared"
	"github.com/golang/mock/gomock"
	"github.com/herumi/bls-eth-go-binary/bls"
	"github.com/stretchr/testify/require"
	"testing"
)

func toBytes(i string) []byte {
	ret, _ := hex.DecodeString(i)
	return ret
}

func mockedSuccessfulAttestationSummary(t *testing.T) (core.IExecutionSummary, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	summary := mocks.NewMockIExecutionSummary(ctrl)
	summary.EXPECT().GetPoolId().Return(uint64(0))

	duty := mocks.NewMockIBeaconDuty(ctrl)
	duty.EXPECT().GetType().Return(uint8(0))
	duty.EXPECT().IsFinalized().Return(true)
	duty.EXPECT().GetParticipation().Return([16]byte{1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0}) // the first executor index is set to 1
	summary.EXPECT().GetDuties().Return([]core.IBeaconDuty{duty})

	return summary, ctrl
}

func GenerateState(t *testing.T) *State {
	require.NoError(t, bls.Init(bls.BLS12_381))
	require.NoError(t, bls.SetETHmode(bls.EthModeDraft07))

	pools := make([]*Pool, 5)

	//
	bps := make([]*BlockProducer, len(pools) * int(core.TestConfig().PoolExecutorsNumber))
	for i := 0 ; i < len(bps) ; i++ {
		sk := &bls.SecretKey{}
		sk.SetByCSPRNG()

		bps[i] = &BlockProducer{
			id:      uint64(i),
			balance: 1000,
			stake:   0,
			slashed: false,
			active:  true,
			pubKey:  sk.GetPublicKey().Serialize(),
		}
	}

	//
	for i := 0 ; i < len(pools) ; i++ {
		executors := make([]uint64, core.TestConfig().PoolExecutorsNumber)
		for j := 0 ; j < int(core.TestConfig().PoolExecutorsNumber) ; j++ {
			executors[j] = bps[i*int(core.TestConfig().PoolExecutorsNumber) + j].GetId()
		} // no need to sort as they are already

		sk := &bls.SecretKey{}
		sk.SetByCSPRNG()

		pools[i] = &Pool{
			id:              uint64(i),
			sortedExecutors: executors,
			pubKey:          sk.GetPublicKey().Serialize(),
		}
	}

	return &State{
		pools: pools,
		//headBlockHeader: &block.BlockHeader{
		//	BlockRoot: nil,
		//	Signature: nil,
		//},
		blockProducers: bps,
		seeds:          [][32]byte{shared.SliceToByte32([]byte("seedseedseedseedseedseedseedseed"))},
	}
}

func TestRandaoSeedMix(t *testing.T) {
	require.NoError(t, bls.Init(bls.BLS12_381))
	require.NoError(t, bls.SetETHmode(bls.EthModeDraft07))

	// mock block body
	ctrl := gomock.NewController(t)
	bodyMock := mocks.NewMockIBlockBody(ctrl)
	bodyMock.EXPECT().GetProposer().Return(uint64(456))
	bodyMock.EXPECT().GetExecutionSummaries().Return(make([]core.IExecutionSummary, 0))
	bodyMock.EXPECT().GetNewPoolRequests().Return(nil)

	// mock header
	sk := &bls.SecretKey{}
	sk.SetByCSPRNG()
	bRoot := []byte("body root body root body root body root body root")
	sig := sk.SignByte(bRoot)
	headerMock := mocks.NewMockIBlockHeader(ctrl)
	headerMock.EXPECT().GetSignature().Return(sig.Serialize())
	headerMock.EXPECT().GetBlockRoot().Return(bRoot)

	state := GenerateState(t)
	newState, err := state.ProcessNewBlock(headerMock, bodyMock)
	require.NoError(t, err)

	expectedSeed,err := shared.MixSeed(state.GetSeed(state.GetCurrentEpoch()), shared.SliceToByte32(sig.Serialize()))
	require.NoError(t, err)
	require.EqualValues(t, expectedSeed, newState.GetSeed(newState.GetCurrentEpoch()))
}

// TODO - test block validation
//func TestBlockValidation(t *testing.T) {
//	require.NoError(t, bls.Init(bls.BLS12_381))
//	require.NoError(t, bls.SetETHmode(bls.EthModeDraft07))
//
//	state := GenerateState(t)
//
//	sk := &bls.SecretKey{}
//	sk.SetByCSPRNG()
//
//	summary, ctrl := mockedSuccessfulAttestationSummary(t)
//	defer ctrl.Finish()
//
//	// generate header and body
//	body,err := block.NewBlockBody(
//			0,
//			0,
//			state,
//			[]core.IExecutionSummary{summary},
//			[]core.ICreatePoolRequest{
//				block.NewCreatePoolRequest(
//					0,0,0,1,0,nil,[16]byte{},
//				),
//			},
//			[]byte("parent"),
//		)
//	require.NoError(t, err)
//	root,err := body.Root()
//	require.NoError(t, err)
//
//	// sign
//	sig := sk.SignByte(root[:])
//
//	head := &block.BlockHeader{
//		BlockRoot: root[:],
//		Signature: sig.Serialize(),
//	}
//
//	// set BP
//	bp := state.GetBlockProducer(0)
//	require.NoError(t, err)
//	bp.SetPubKey(sk.GetPublicKey())
//
//	require.NoError(t, state.ValidateBlock(head, body))
//}

func TestCreatedNewPoolReq(t *testing.T) {
	require.NoError(t, bls.Init(bls.BLS12_381))
	require.NoError(t, bls.SetETHmode(bls.EthModeDraft07))

	sk := &bls.SecretKey{}
	sk.SetByCSPRNG()

	reqs := []core.ICreatePoolRequest{
		block.NewCreatePoolRequest(
			0,
			1,
			0,
			1,
			0,
			sk.GetPublicKey().Serialize(),
			[16]byte{1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0}, // pos 0 is set only,
		),
	}

	state := GenerateState(t)
	currentBP := state.GetBlockProducer(0)

	participants,err := state.DKGCommittee(0,0)

	require.NoError(t, state.ProcessNewPoolRequests(reqs))
	require.Equal(t, 6, len(state.pools))

	// check new balances
	currentBP = state.GetBlockProducer(currentBP.GetId())
	require.NoError(t, err)
	require.EqualValues(t, uint64(4000), currentBP.GetBalance())

	bp := state.GetBlockProducer(participants[0])
	require.EqualValues(t, uint64(2000), bp.GetBalance())

	for i := 1 ; i < len(participants) ; i++ {
		bp := state.GetBlockProducer(participants[i])
		if bp.GetId() == currentBP.GetId() {
			continue // because he is the leader
		}
		require.EqualValues(t, uint64(0), bp.GetBalance())
	}
}

func TestFailedToCreateNewPool(t *testing.T) {
	require.NoError(t, bls.Init(bls.BLS12_381))
	require.NoError(t, bls.SetETHmode(bls.EthModeDraft07))

	sk := &bls.SecretKey{}
	sk.SetByCSPRNG()

	reqs := []core.ICreatePoolRequest{
		block.NewCreatePoolRequest(
			0,
			2,
			0,
			1,
			0,
			sk.GetPublicKey().Serialize(),
			[16]byte{1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0}, // random assignments
		),
	}

	state := GenerateState(t)
	currentBP := state.GetBlockProducer(0)

	participants, err := state.DKGCommittee(0, 0)
	require.NoError(t, err)

	require.NoError(t, state.ProcessNewPoolRequests(reqs))
	require.Equal(t, 5, len(state.pools))

	// check new balances
	currentBP = state.GetBlockProducer(currentBP.GetId())
	require.EqualValues(t, 1000, currentBP.GetBalance())

	for i := 0; i < len(participants); i++ {
		bp := state.GetBlockProducer(participants[i])
		require.EqualValues(t, 0, bp.GetBalance())
	}
}

func TestStateSSZ(t *testing.T) {
	tests := []struct {
		name string
		state *State
		expected []byte
	}{
		{
			name: "full SSZ",
			state:NewState(
					1234562,
					[]*Pool{
						NewPool(
							12,
							true,
							[]byte{1,2,3,4,5},
							[]uint64{12,5423,1245,12435,21,0},
							),
					},
					12,
					[]*BlockProducer{
						NewBlockProducer(
							12,
							[]byte{1,2,3,4,5},
							100,
							100,
							false,
							true,
							),
					},
					[32]byte{1,2,3,4,5},
				),
			expected:toBytes("893981688d75771da9898f8370c4c4458a0157d8ff7f974979f8f42e40247cc4"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func (t *testing.T) {
			root, err := test.state.Root()
			require.NoError(t, err)
			require.EqualValues(t, test.expected, root[:])
		})
	}
}