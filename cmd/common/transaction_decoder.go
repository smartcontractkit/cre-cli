package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

type DecodedTxInputs map[string]interface{}

// NewDecodedTxInputs sanitizes input maps, converting byte arrays to hex strings.
func NewDecodedTxInputs(inputMap map[string]interface{}) DecodedTxInputs {
	sanitized := make(DecodedTxInputs)

	for key, value := range inputMap {
		val := reflect.ValueOf(value)
		switch val.Kind() {
		case reflect.Array:
			if val.Type().Elem().Kind() == reflect.Uint8 {
				byteSlice := make([]byte, val.Len())
				for i := 0; i < val.Len(); i++ {
					byteSlice[i] = byte(val.Index(i).Uint())
				}
				sanitized[key] = hex.EncodeToString(byteSlice)
			} else {
				sanitized[key] = value
			}
		case reflect.Slice:
			if b, ok := value.([]byte); ok {
				sanitized[key] = hex.EncodeToString(b)
			} else if val.Type().Elem().Kind() == reflect.Array && val.Type().Elem().Elem().Kind() == reflect.Uint8 {
				var hexStrings []string
				for i := 0; i < val.Len(); i++ {
					arrayVal := val.Index(i)
					byteSlice := make([]byte, arrayVal.Len())
					for j := 0; j < arrayVal.Len(); j++ {
						byteSlice[j] = byte(arrayVal.Index(j).Uint())
					}
					hexStrings = append(hexStrings, hex.EncodeToString(byteSlice))
				}
				sanitized[key] = hexStrings
			} else {
				sanitized[key] = value
			}
		default:
			sanitized[key] = value
		}
	}
	return sanitized
}

func FindAbiAndDecodeTxInputs(l *zerolog.Logger, s *seth.Client, tx *types.Transaction) (string, DecodedTxInputs, error) {
	if len(tx.Data()) < 4 {
		return "", nil, fmt.Errorf("tx data is less than 4 bytes, can't decode tx: %+v", tx)
	}
	abiResult, err := s.ABIFinder.FindABIByMethod(tx.To().Hex(), tx.Data()[:4])
	if err != nil {
		return "", nil, err
	}

	decodedTxInputs, err := decodeTxInputs(l, tx.Data(), abiResult.Method)
	if err != nil {
		return "", nil, err
	}
	return abiResult.Method.Name, decodedTxInputs, nil
}

func decodeTxInputs(l *zerolog.Logger, txData []byte, method *abi.Method) (DecodedTxInputs, error) {
	l.Debug().Bytes("Transaction data", txData).Msg("Parsing transaction inputs")
	if (len(txData)) < 4 {
		return nil, errors.New("tx data is less than 4 bytes, can't decode")
	}

	inputMap := make(map[string]interface{})
	payload := txData[4:]
	if len(payload) == 0 || len(method.Inputs) == 0 {
		return nil, nil
	}
	err := method.Inputs.UnpackIntoMap(inputMap, payload)
	if err != nil {
		return nil, err
	}

	sanitizedInputs := NewDecodedTxInputs(inputMap)

	l.Debug().Interface("Inputs", sanitizedInputs).Msg("Transaction inputs")
	return sanitizedInputs, nil
}
