package files

import (
	"fmt"
	"os"
	"reflect"

	"github.com/go-playground/validator/v10"
)

func HasReadAccessToPath(fl validator.FieldLevel) bool {
	field := fl.Field()

	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	path := field.String()

	// Check if the file or directory exists
	_, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Attempt to open the file or directory to verify read access
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	return true
}
