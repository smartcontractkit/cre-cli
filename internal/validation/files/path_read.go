package files

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

func HasReadAccessToPath(fl validator.FieldLevel) bool {
	path := mustBeString(fl)
	return checkReadAccess(path)
}

func checkReadAccess(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	return true
}

// HasReadAccessToWorkflowPath validates workflow-path: for .wasm paths only the containing
// directory must exist (CompileWorkflowToWasm will run make build); otherwise same as path_read.
func HasReadAccessToWorkflowPath(fl validator.FieldLevel) bool {
	path := mustBeString(fl)
	if strings.HasSuffix(path, ".wasm") {
		return checkReadAccess(filepath.Dir(path))
	}
	return checkReadAccess(path)
}

func mustBeString(fl validator.FieldLevel) string {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}
	return field.String()
}
