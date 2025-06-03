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

func goStructTypeToABIType(t reflect.Type) (abi.Type, error) {
	if t.Kind() != reflect.Struct {
		return abi.Type{}, errors.New("input must be a struct type")
	}
	components := []abi.ArgumentMarshaling{}
	names := []string{}
	for i := range t.NumField() {
		field := t.Field(i)
		fieldTyp := field.Type
		fieldName := field.Name
		abiType, err := goTypeToABIType(fieldTyp)
		abiTypeStr := abiType.String()
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

func tupleTypeToArgumentMarshaling(t *abi.Type) ([]abi.ArgumentMarshaling, error) {
	if t.T != abi.TupleTy {
		return nil, fmt.Errorf("expected TupleTy, got %v", t.T)
	}
	args := make([]abi.ArgumentMarshaling, len(t.TupleElems))
	for i, elem := range t.TupleElems {
		arg := abi.ArgumentMarshaling{
			Name: "",
			Type: elem.String(),
		}
		if len(t.TupleRawNames) > i {
			arg.Name = t.TupleRawNames[i]
		}
		// Recursively convert nested tuples
		if elem.T == abi.TupleTy {
			nested, err := tupleTypeToArgumentMarshaling(elem)
			if err != nil {
				return nil, err
			}
			arg.Type = "tuple"
			arg.Components = nested
		} else if elem.T == abi.SliceTy || elem.T == abi.ArrayTy {
			if elem.Elem.T == abi.TupleTy {
				nested, err := tupleTypeToArgumentMarshaling(elem.Elem)
				if err != nil {
					return nil, err
				}
				arg.Type = elem.String() // example: tuple[], tuple[3]
				arg.Components = nested
			}
		}
		args[i] = arg
	}
	return args, nil
}

func canonicalTypeString(t *abi.Type) string {
	switch t.T {
	case abi.TupleTy:
		elems := make([]string, len(t.TupleElems))
		for i, e := range t.TupleElems {
			elems[i] = canonicalTypeString(e)
		}
		return fmt.Sprintf("tuple(%s)", strings.Join(elems, ","))
	case abi.SliceTy:
		return canonicalTypeString(t.Elem) + "[]"
	case abi.ArrayTy:
		return fmt.Sprintf("%s[%d]", canonicalTypeString(t.Elem), t.Size)
	default:
		return t.String()
	}
}

func goTypeToABIType(typ reflect.Type) (abi.Type, error) {
	switch typ.Kind() {
	case reflect.Int, reflect.Uint:
		return abi.Type{}, fmt.Errorf("ints must have explicit size, type not valid: %s", typ)
	case reflect.Bool, reflect.String, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return abi.NewType(strings.ToLower(typ.Kind().String()), "", nil)
	case reflect.Array: // example) address, [3]uint, [5]GameSearchResult
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
		abiType, err := goTypeToABIType(typ.Elem())
		if err != nil {
			return abi.Type{}, fmt.Errorf("unrecognized slice-elem type: %w", err)
		}
		elemTyp := abiType.String()
		if abiType.TupleType != nil {
			internalTyp, err := goStructTypeToABIType(abiType.TupleType)
			if err != nil {
				return abi.Type{}, fmt.Errorf("struct conversion failure: type %s: %w", abiType, err)
			}
			component, err := tupleTypeToArgumentMarshaling(&internalTyp)
			if err != nil {
				return abi.Type{}, fmt.Errorf("argument marshalling failure: type %s: %w", internalTyp.String(), err)
			}
			return abi.NewType(fmt.Sprintf("tuple%s[%d]", elemTyp, typ.Len()), "", component)
		}
		return abi.NewType(fmt.Sprintf("%s[%d]", elemTyp, typ.Len()), "", nil)
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return abi.NewType("bytes", "", nil)
		}
		abiType, err := goTypeToABIType(typ.Elem())
		if err != nil {
			return abi.Type{}, fmt.Errorf("unrecognized slice-elem type: %w", err)
		}
		elemTyp := abiType.String()
		if abiType.TupleType != nil { // example: []GameSearchResult
			internalTyp, err := goStructTypeToABIType(abiType.TupleType)
			if err != nil {
				return abi.Type{}, fmt.Errorf("struct conversion failure: type %s: %w", abiType, err)
			}
			component, err := tupleTypeToArgumentMarshaling(&internalTyp)
			if err != nil {
				return abi.Type{}, fmt.Errorf("argument marshalling failure: type %s: %w", internalTyp.String(), err)
			}
			return abi.NewType(fmt.Sprintf("tuple%s[]", elemTyp), "", component)
		}
		// example: []bytes32
		return abi.NewType(elemTyp+"[]", "", nil)
	case reflect.Struct:
		if typ.AssignableTo(abiInt256Type) {
			return abi.NewType("int256", "", nil)
		}
		if typ.ConvertibleTo(reflect.TypeFor[big.Int]()) {
			return abi.NewType("uint256", "", nil)
		}
		abiType, err := goStructTypeToABIType(typ)
		if err != nil {
			return abi.Type{}, fmt.Errorf("struct conversion failure: type %s: %w", typ, err)
		}
		return abiType, err
	case reflect.Pointer:
		abiType, err := goTypeToABIType(typ.Elem())
		if err != nil {
			return abi.Type{}, fmt.Errorf("unrecognized pointer-elem type: %w", err)
		}
		return abiType, nil
	default:
		return abi.Type{}, fmt.Errorf("unrecognized typ: %s", typ)
	}
}
