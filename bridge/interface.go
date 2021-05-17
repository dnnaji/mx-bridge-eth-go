package bridge

import (
	"context"
)

type Broadcaster interface {
	Signatures() [][]byte
	SendSignature(signature []byte)
}

type Bridge interface {
	GetPendingDepositTransaction(context.Context) *DepositTransaction
	ProposeTransfer(context.Context, *DepositTransaction) (string, error)
	ProposeSetStatus(context.Context, uint8, Nonce)
	WasProposedTransfer(context.Context, Nonce) bool
	GetActionIdForProposeTransfer(context.Context, Nonce) ActionId
	WasProposedSetStatusSuccessOnPendingTransfer(context.Context) bool
	WasProposedSetStatusFailedOnPendingTransfer(context.Context) bool
	GetActionIdForSetStatusOnPendingTransfer(context.Context) ActionId
	WasExecuted(context.Context, ActionId, Nonce) bool
	Sign(context.Context, ActionId) (string, error)
	Execute(context.Context, ActionId, Nonce) (string, error)
	SignersCount(context.Context, ActionId) uint
}