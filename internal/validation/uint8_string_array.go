package validation

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

func isUint8Array(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	decimalsArray := fl.Field().String()

	if decimalsArray == "" {
		return false
	}

	decimalsArr := strings.Trim(decimalsArray, "[ ]")
	decimalsStrArr := strings.Split(decimalsArr, ",")
	for _, d := range decimalsStrArr {
		d = strings.TrimSpace(d)
		_, err := strconv.ParseUint(d, 10, 8)
		if err != nil {
			return false
		}
	}

	return true
}
