package validation

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-playground/validator/v10"
)

func isECDSAPrivateKey(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	privateKey := fl.Field().String()

	if privateKey == "" {
		return false
	}

	if match, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", privateKey); !match {
		return false
	}

	privateKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return false
	}

	if _, err = crypto.ToECDSA(privateKeyBytes); err != nil {
		return false
	}

	return true
}
