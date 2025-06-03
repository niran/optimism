package bindings

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/holiman/uint256"
)

type ABIInt256 big.Int

var abiInt256Type = reflect.TypeFor[ABIInt256]()

var abiUint256Type = reflect.TypeFor[uint256.Int]()

func goStructTypeToSolidityType(t reflect.Type) (abi.Type, error) {
	if t.Kind() != reflect.Struct {
		return abi.Type{}, errors.New("input must be a struct type")
	}
	components := []abi.ArgumentMarshaling{}
	names := []string{}
	for i := range t.NumField() {
		field := t.Field(i)
		fieldTyp := field.Type
		fieldName := field.Name
		abiTypeStr, _, err := goTypeToSolidityType(fieldTyp)
		if err != nil {
			return abi.Type{}, fmt.Errorf("field %s: %w", fieldName, err)
		}
		names = append(names, fieldName)
		components = append(components, abi.ArgumentMarshaling{
			Name: fieldName, Type: abiTypeStr,
		})
	}
	tuple, err := abi.NewType("tuple", "", components)
	if err != nil {
		return abi.Type{}, fmt.Errorf("failed to construct tuple: %w", err)
	}
	tuple.TupleRawNames = names
	return tuple, nil
}

func goTypeToSolidityType(typ reflect.Type) (abi.Type, error) {
	switch typ.Kind() {
	case reflect.Int, reflect.Uint:
		return abi.Type{}, fmt.Errorf("ints must have explicit size, type not valid: %s", typ)
	case reflect.Bool, reflect.String, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return abi.NewType(strings.ToLower(typ.Kind().String()), "", nil)
	case reflect.Array:
		if typ.AssignableTo(abiUint256Type) { // uint256.Int underlying Go type is [4]uint64
			return abi.NewType("uint256", "", nil)
		}
		if typ.Elem().Kind() == reflect.Uint8 {
			if typ.Len() == 20 && typ.Name() == "Address" {
				return abi.NewType("address", "", nil)
			}
			if typ.Len() > 32 {
				return abi.Type{}, fmt.Errorf("byte array too large: %d", typ.Len())
			}
			return abi.NewType(fmt.Sprintf("bytes%d", typ.Len()), "", nil)
		}
		elemTyp, internalTyp, err := goTypeToSolidityType(typ.Elem())
		if err != nil {
			return abi.Type{}, fmt.Errorf("unrecognized slice-elem type: %w", err)
		}
		if internalTyp != "" {
			return abi.Type{}, fmt.Errorf("nested internal types not supported: %w", err)
		}
		return abi.NewType(fmt.Sprintf("%s[%d]", elemTyp, typ.Len()), "", nil)
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return abi.NewType("bytes", "", nil)
		}
		elemABITyp, internalTyp, err := goTypeToSolidityType(typ.Elem())
		if err != nil {
			return abi.Type{}, fmt.Errorf("unrecognized slice-elem type: %w", err)
		}
		if internalTyp != "" {
			return abi.Type{}, fmt.Errorf("nested internal types not supported: %w", err)
		}
		return elemABITyp + "[]", "", nil
	case reflect.Struct:
		if typ.AssignableTo(abiInt256Type) {
			return abi.NewType("int256", "", nil)
		}
		if typ.ConvertibleTo(reflect.TypeFor[big.Int]()) {
			return abi.NewType("uint256", "", nil)
		}
		abiType, err := goStructTypeToSolidityType(typ)
		if err != nil {
			return abi.Type{}, fmt.Errorf("struct conversion failure, cannot handle type %s: %w", typ, err)
		}
		return abiType, err
		// We can parse into abi.TupleTy in the future, if necessary
	case reflect.Pointer:
		elemABITyp, internalTyp, err := goTypeToSolidityType(typ.Elem())
		if err != nil {
			return abi.Type{}, fmt.Errorf("unrecognized pointer-elem type: %w", err)
		}
		return elemABITyp, internalTyp, nil
	default:
		return abi.Type{}, fmt.Errorf("unrecognized typ: %s", typ)
	}
}
