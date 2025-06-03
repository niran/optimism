package bindings

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type TestSimpleStructA struct {
	a *big.Int
	b []byte
	c common.Address
}

type TestSimpleStructB struct {
	a [3]byte
	b [32]byte
	c *uint256.Int
}

type TestNestedStruct struct {
	a TestSimpleStructA
	b TestSimpleStructB
	c [3]TestSimpleStructA
}

type TestComplexStruct struct {
	a TestSimpleStructB
	b []TestNestedStruct
	c TestSimpleStructA
	d *big.Int
	e TestSimpleStructB
	f TestSimpleStructA
	g [5]TestNestedStruct
	h []byte
	i [5]byte
}

type TestRecursiveStruct struct {
	a TestNestedStruct
}

type TestRecursiveStruct2 struct {
	a TestRecursiveStruct
}

type TestRecursiveStruct3 struct {
	a TestRecursiveStruct2
}

func TestTypeConversion(t *testing.T) {
	type testCase struct {
		value    any
		want     string
		testName string
	}

	tests := []testCase{
		{
			value:    TestSimpleStructA{},
			want:     "(uint256,bytes,address)",
			testName: "SimpleStructA (value)",
		},
		{
			value:    &TestSimpleStructA{},
			want:     "(uint256,bytes,address)",
			testName: "SimpleStructA (pointer)",
		},
		{
			value:    TestSimpleStructB{},
			want:     "(bytes3,bytes32,uint256)",
			testName: "SimpleStructB",
		},
		{
			value:    TestNestedStruct{},
			want:     "((uint256,bytes,address),(bytes3,bytes32,uint256),(uint256,bytes,address)[3])",
			testName: "NestedStruct",
		},
		{
			value:    TestRecursiveStruct2{},
			want:     "((((uint256,bytes,address),(bytes3,bytes32,uint256),(uint256,bytes,address)[3])))",
			testName: "RecursiveStruct2",
		},
		{
			value:    TestRecursiveStruct3{},
			want:     "(((((uint256,bytes,address),(bytes3,bytes32,uint256),(uint256,bytes,address)[3]))))",
			testName: "RecursiveStruct3",
		},
		{
			value:    &TestRecursiveStruct3{},
			want:     "(((((uint256,bytes,address),(bytes3,bytes32,uint256),(uint256,bytes,address)[3]))))",
			testName: "RecursiveStruct3 (pointer)",
		},
		{
			value:    TestComplexStruct{},
			want:     "((bytes3,bytes32,uint256),(uint256,bytes3,uint256[3])[3])[],(uint256,bytes,address),uint256,(bytes3,bytes32,uint256),(uint256,bytes,address),(uint256,bytes3,uint256[3])[3])[5],bytes,bytes5)",
			testName: "ComplexStruct",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			typ, err := goTypeToABIType(reflect.TypeOf(tc.value))
			require.NoError(t, err)
			require.Equal(t, tc.want, typ.String())
		})
	}
}
