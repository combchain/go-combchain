package types

import (
	"github.com/combchain/go-combchain/vm/avm/interfaces"
	"math/big"
)

type StackItemInterface interface {
	Equals(other StackItemInterface) bool
	GetBigInteger() *big.Int
	GetBoolean() bool
	GetByteArray() []byte
	GetInterface() interfaces.IInteropInterface
	GetArray() []StackItemInterface
}
