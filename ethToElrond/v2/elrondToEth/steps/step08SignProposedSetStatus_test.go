package steps

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/elrond-eth-bridge/clients"
	"github.com/ElrondNetwork/elrond-eth-bridge/core"
	v2 "github.com/ElrondNetwork/elrond-eth-bridge/ethToElrond/v2"
	"github.com/ElrondNetwork/elrond-eth-bridge/ethToElrond/v2/elrondToEth"
	"github.com/ElrondNetwork/elrond-eth-bridge/testsCommon/bridgeV2"
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/stretchr/testify/assert"
)

var actionID = uint64(662528)

func TestExecute_SignProposedSetStatus(t *testing.T) {
	t.Parallel()
	t.Run("nil batch on GetStoredBatch", func(t *testing.T) {
		t.Parallel()
		bridgeStub := createStubExecutorSignProposedSetStatus()
		bridgeStub.GetStoredBatchCalled = func() *clients.TransferBatch {
			return nil
		}

		step := signProposedSetStatusStep{
			bridge: bridgeStub,
		}

		stepIdentifier, err := step.Execute(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, initialStep, stepIdentifier)
	})
	t.Run("error on GetAndStoreActionIDForProposeSetStatusFromElrond", func(t *testing.T) {
		t.Parallel()
		bridgeStub := createStubExecutorSignProposedSetStatus()
		bridgeStub.GetAndStoreActionIDForProposeSetStatusFromElrondCalled = func(ctx context.Context) (uint64, error) {
			return v2.InvalidActionID, expectedError
		}

		step := signProposedSetStatusStep{
			bridge: bridgeStub,
		}

		stepIdentifier, err := step.Execute(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, initialStep, stepIdentifier)
	})
	t.Run("invalid actionID on GetAndStoreActionIDForProposeSetStatusFromElrond", func(t *testing.T) {
		t.Parallel()
		bridgeStub := createStubExecutorSignProposedSetStatus()
		bridgeStub.GetAndStoreActionIDForProposeSetStatusFromElrondCalled = func(ctx context.Context) (uint64, error) {
			return v2.InvalidActionID, nil
		}

		step := signProposedSetStatusStep{
			bridge: bridgeStub,
		}

		stepIdentifier, err := step.Execute(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, initialStep, stepIdentifier)
	})
	t.Run("error on WasActionSignedOnElrond", func(t *testing.T) {
		t.Parallel()
		bridgeStub := createStubExecutorSignProposedSetStatus()
		bridgeStub.WasActionSignedOnElrondCalled = func(ctx context.Context) (bool, error) {
			return false, expectedError
		}

		step := signProposedSetStatusStep{
			bridge: bridgeStub,
		}

		stepIdentifier, err := step.Execute(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, initialStep, stepIdentifier)
	})
	t.Run("error on SignActionOnElrond", func(t *testing.T) {
		t.Parallel()
		bridgeStub := createStubExecutorSignProposedSetStatus()
		bridgeStub.SignActionOnElrondCalled = func(ctx context.Context) error {
			return expectedError
		}

		step := signProposedSetStatusStep{
			bridge: bridgeStub,
		}

		stepIdentifier, err := step.Execute(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, initialStep, stepIdentifier)
	})
	t.Run("should work", func(t *testing.T) {
		t.Parallel()
		t.Run("if proposed set status was signed, go to WaitingForQuorumOnSetStatus", func(t *testing.T) {
			t.Parallel()
			bridgeStub := createStubExecutorSignProposedSetStatus()
			bridgeStub.WasActionSignedOnElrondCalled = func(ctx context.Context) (bool, error) {
				return true, nil
			}

			wasCalled := false
			bridgeStub.SignActionOnElrondCalled = func(ctx context.Context) error {
				wasCalled = true
				return nil
			}

			step := signProposedSetStatusStep{
				bridge: bridgeStub,
			}

			expectedStep := core.StepIdentifier(elrondToEth.WaitingForQuorumOnSetStatus)
			stepIdentifier, err := step.Execute(context.Background())
			assert.Nil(t, err)
			assert.False(t, wasCalled)
			assert.Equal(t, expectedStep, stepIdentifier)
		})
		t.Run("if proposed set status was not signed, sign and go to WaitingForQuorumOnSetStatus", func(t *testing.T) {
			t.Parallel()
			bridgeStub := createStubExecutorSignProposedSetStatus()
			wasCalled := false
			bridgeStub.SignActionOnElrondCalled = func(ctx context.Context) error {
				wasCalled = true
				return nil
			}

			step := signProposedSetStatusStep{
				bridge: bridgeStub,
			}

			assert.False(t, step.IsInterfaceNil())
			expectedStep := core.StepIdentifier(elrondToEth.WaitingForQuorumOnSetStatus)
			stepIdentifier, err := step.Execute(context.Background())
			assert.Nil(t, err)
			assert.True(t, wasCalled)
			assert.NotEqual(t, step.Identifier(), stepIdentifier)
			assert.Equal(t, expectedStep, stepIdentifier)
		})
	})

}

func createStubExecutorSignProposedSetStatus() *bridgeV2.BridgeExecutorStub {
	stub := bridgeV2.NewBridgeExecutorStub()
	stub.GetLoggerCalled = func() logger.Logger {
		return testLogger
	}
	stub.GetStoredBatchCalled = func() *clients.TransferBatch {
		return testBatch
	}
	stub.GetAndStoreActionIDForProposeSetStatusFromElrondCalled = func(ctx context.Context) (uint64, error) {
		return actionID, nil
	}
	stub.WasActionSignedOnElrondCalled = func(ctx context.Context) (bool, error) {
		return false, nil
	}
	stub.SignActionOnElrondCalled = func(ctx context.Context) error {
		return nil
	}
	return stub
}