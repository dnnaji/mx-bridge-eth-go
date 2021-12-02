package clients

import (
	"fmt"
	"math/big"
)

// TransferBatch is the transfer batch structure agnostic of any chain implementation
type TransferBatch struct {
	ID       uint64
	Deposits []*DepositTransfer
}

// Clone will deep clone the current TransferBatch instance
func (tb *TransferBatch) Clone() *TransferBatch {
	cloned := &TransferBatch{
		ID:       tb.ID,
		Deposits: make([]*DepositTransfer, 0, len(tb.Deposits)),
	}

	for _, dt := range tb.Deposits {
		cloned.Deposits = append(cloned.Deposits, dt.Clone())
	}

	return cloned
}

// String will convert the transfer batch to a string
func (tb *TransferBatch) String() string {
	str := fmt.Sprintf("Batch id %d:", tb.ID)
	for _, dt := range tb.Deposits {
		str += "\n  " + dt.String()
	}

	return str
}

// DepositTransfer is the deposit transfer structure agnostic of any chain implementation
type DepositTransfer struct {
	Nonce            uint64
	ToBytes          []byte
	DisplayableTo    string
	FromBytes        []byte
	DisplayableFrom  string
	TokenBytes       []byte
	DisplayableToken string
	Amount           *big.Int
}

// String will convert the deposit transfer to a string
func (dt *DepositTransfer) String() string {
	return fmt.Sprintf("to: %s, from: %s, token address: %s, amount: %v, deposit nonce: %d",
		dt.DisplayableTo, dt.DisplayableFrom, dt.DisplayableToken, dt.Amount, dt.Nonce)
}

// Clone will deep clone the current DepositTransfer instance
func (dt *DepositTransfer) Clone() *DepositTransfer {
	cloned := &DepositTransfer{
		Nonce:            dt.Nonce,
		ToBytes:          make([]byte, len(dt.ToBytes)),
		DisplayableTo:    dt.DisplayableTo,
		FromBytes:        make([]byte, len(dt.FromBytes)),
		DisplayableFrom:  dt.DisplayableFrom,
		TokenBytes:       make([]byte, len(dt.TokenBytes)),
		DisplayableToken: dt.DisplayableToken,
		Amount:           big.NewInt(0),
	}

	copy(cloned.ToBytes, dt.ToBytes)
	copy(cloned.FromBytes, dt.FromBytes)
	copy(cloned.TokenBytes, dt.TokenBytes)
	if dt.Amount != nil {
		cloned.Amount.Set(dt.Amount)
	}

	return cloned
}