package validation

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"

	"github.com/smartcontractkit/cre-cli/internal/validation/files"
)

var customValidators = map[string]validator.Func{
	"ecdsa_private_key":  isECDSAPrivateKey,
	"uint8_string_array": isUint8Array,
	"json":               files.IsValidJSON,
	"path_read":          files.HasReadAccessToPath,
	"wasm":               files.IsValidWASM,
	"workflow_name":      isWorkflowName,
	"workflow_owner":     isWorkflowOwner,
	"yaml":               files.IsValidYAML,
}

var customTranslations = map[string]string{
	"ascii":              "{0} must contain only ASCII characters: {1}",
	"dir":                "{0} must be a valid existing directory: {1}",
	"ecdsa_private_key":  "{0} must be a valid ECDSA private key",
	"eth_addr":           "{0} must be a valid ethereum address: {1}",
	"file":               "{0} must be a valid existing file: {1}",
	"filepath":           "{0} must be a valid file path: {1}",
	"github_token":       "{0} must be a valid GitHub token able to read and write Gists",
	"http_url":           "{0} must be a valid HTTP URL: {1}",
	"http_url|eq=":       "{0} must be empty or a valid HTTP URL: {1}",
	"json":               "{0} must be a valid JSON file: {1}",
	"path_read":          "{0} must have read access to path: {1}",
	"wasm":               "{0} must be a valid WASM file: {1}",
	"workflow_name":      "{0} must be non-empty and no longer than 64 characters: {1}",
	"workflow_owner":     "{0} must be a valid Ethereum address starting with 0x: {1}",
	"yaml":               "{0} must be a valid YAML file: {1}",
	"uint8_string_array": "{0} must be a valid uint8 array wrapped within brackets: {1}",
}

type ValidationError struct {
	Field  string
	Detail string
}

type ValidationErrors []ValidationError

func NewValidationError(key, detail string) error {
	return &ValidationError{
		Field:  key,
		Detail: detail,
	}
}

func (e *ValidationError) Error() string {
	return e.Detail
}

// Add error interface implementation for ValidationErrors
func (ve ValidationErrors) Error() string {
	msg := "validation error\n"
	for _, err := range ve {
		msg += err.Detail + "\n"
	}
	return msg
}

// Validator wraps a validator instance and a translator.
type Validator struct {
	validate *validator.Validate
	trans    ut.Translator
}

// NewValidator creates a new Validator with English translations registered.
func NewValidator() (*Validator, error) {
	validate := validator.New()

	// Use the "cli" tag to override the field name if present.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		cliTag := fld.Tag.Get("cli")
		if cliTag != "" {
			return cliTag
		}
		return fld.Name
	})

	enLocale := en.New()
	uni := ut.New(enLocale, enLocale)
	trans, found := uni.GetTranslator("en")
	if !found {
		return nil, errors.New("translator not found")
	}

	if err := registerDefaultTranslations(validate, trans); err != nil {
		return nil, err
	}

	for validatorName, validatorFunc := range customValidators {
		if err := validate.RegisterValidation(validatorName, validatorFunc); err != nil {
			return nil, err
		}
	}

	return &Validator{validate: validate, trans: trans}, nil
}

// RegisterCustomTranslation registers a custom translation for a given tag.
func (v *Validator) RegisterCustomTranslation(tag, msg string) error {
	return v.validate.RegisterTranslation(tag, v.trans,
		func(ut ut.Translator) error {
			// The first argument {0} in the message template represents the field name.
			return ut.Add(tag, msg, true)
		},
		func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T(tag, fe.Field(), fmt.Sprintf("%v", fe.Value()))
			return t
		},
	)
}

// Struct validates a struct and returns translated errors if any.
func (v *Validator) Struct(s interface{}) error {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		// Collect translated errors
		var msg string
		for _, e := range verrs {
			msg += e.Translate(v.trans) + "\n"
		}
		// Wrap the original error with the translated message
		return fmt.Errorf("validation error:\n%s: %w", msg, verrs)
	}
	return err
}

// Validate returns the underlying *validator.Validate instance if you need it directly.
func (v *Validator) Validate() *validator.Validate {
	return v.validate
}

// Translator returns the underlying translator if you need direct access.
func (v *Validator) Translator() ut.Translator {
	return v.trans
}

// ParseValidationErrors parses a raw validation error and returns a slice of ValidationErrors.
func (v *Validator) ParseValidationErrors(err error) ValidationErrors {
	ves := ValidationErrors{}

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, verr := range verrs {
			ves = append(ves, ValidationError{
				Field:  verr.StructNamespace(),
				Detail: verr.Translate(v.trans),
			})
		}
	}

	return ves
}

func registerDefaultTranslations(v *validator.Validate, trans ut.Translator) error {
	// Register default translations for all built-in tags
	if err := en_translations.RegisterDefaultTranslations(v, trans); err != nil {
		return fmt.Errorf("failed to register default translations: %w", err)
	}

	for tag, message := range customTranslations {
		if err := v.RegisterTranslation(tag, trans,
			func(ut ut.Translator) error {
				return ut.Add(tag, message, true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tag, fe.Field(), fmt.Sprintf("%v", fe.Value()))
				return t
			},
		); err != nil {
			return fmt.Errorf("failed to register custom translation for %s: %w", tag, err)
		}
	}

	return nil
}
