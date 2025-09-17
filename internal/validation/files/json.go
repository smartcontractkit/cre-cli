package files

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/go-playground/validator/v10"
)

func IsValidJSON(fl validator.FieldLevel) bool {
	field := fl.Field()

	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}
	path := field.String()

	// Check if the file exists
	fileInfo, err := os.Stat(path)
	if err != nil || fileInfo.IsDir() {
		return false
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Decode JSON content
	var content interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&content); err != nil {
		return false
	}

	return true
}
