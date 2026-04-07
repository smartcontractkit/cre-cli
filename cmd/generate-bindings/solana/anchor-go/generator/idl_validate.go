package generator

import (
	"fmt"
	"strings"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/tools"
)

// ValidateIDLDerivedIdentifiers checks that names from the IDL produce valid Go identifiers
// after the same transforms used by the Jennifer-based generator. Call this before Generate().
func ValidateIDLDerivedIdentifiers(i *idl.Idl) error {
	if i == nil {
		return fmt.Errorf("idl is nil")
	}
	for ai, acc := range i.Accounts {
		ctx := fmt.Sprintf("accounts[%d](name=%q)", ai, acc.Name)
		if err := validatePascalIdent(ctx, acc.Name); err != nil {
			return err
		}
		disc := FormatAccountDiscriminatorName(acc.Name)
		if err := validateRawIdent(ctx+".discriminatorVar", acc.Name, disc); err != nil {
			return err
		}
	}
	for ei, ev := range i.Events {
		ctx := fmt.Sprintf("events[%d](name=%q)", ei, ev.Name)
		if err := validatePascalIdent(ctx, ev.Name); err != nil {
			return err
		}
		disc := FormatEventDiscriminatorName(ev.Name)
		if err := validateRawIdent(ctx+".discriminatorVar", ev.Name, disc); err != nil {
			return err
		}
	}
	for ci, co := range i.Constants {
		if co.Name == "" {
			continue
		}
		ctx := fmt.Sprintf("constants[%d]", ci)
		if err := validateRawIdent(ctx, co.Name, co.Name); err != nil {
			return err
		}
	}
	for ixIdx, ix := range i.Instructions {
		ctx := fmt.Sprintf("instructions[%d](name=%q)", ixIdx, ix.Name)
		if err := validatePascalIdent(ctx, ix.Name); err != nil {
			return err
		}
		disc := FormatInstructionDiscriminatorName(ix.Name)
		if err := validateRawIdent(ctx+".discriminatorVar", ix.Name, disc); err != nil {
			return err
		}
		fn := newInstructionFuncName(ix.Name)
		if err := validateRawIdent(ctx+".constructor", ix.Name, fn); err != nil {
			return err
		}
		typeName := instructionStructTypeName(ix.Name)
		if err := validateRawIdent(ctx+".instructionStructType", ix.Name, typeName); err != nil {
			return err
		}
		for _, arg := range ix.Args {
			argCtx := ctx + ".args(name=" + quoteIDL(arg.Name) + ")"
			if err := validatePascalIdent(argCtx, arg.Name); err != nil {
				return err
			}
			param := formatParamName(arg.Name)
			if err := validateRawIdent(argCtx+".builderParam", arg.Name, param); err != nil {
				return err
			}
		}
		for ai, accItem := range ix.Accounts {
			switch acc := accItem.(type) {
			case *idl.IdlInstructionAccount:
				acCtx := fmt.Sprintf("%s.accounts[%d](name=%q)", ctx, ai, acc.Name)
				if err := validatePascalIdent(acCtx, acc.Name); err != nil {
					return err
				}
				fieldBase := tools.ToCamelUpper(acc.Name)
				if err := validateRawIdent(acCtx+".accountField", acc.Name, fieldBase); err != nil {
					return err
				}
				if acc.Writable {
					if err := validateRawIdent(acCtx+".writableFlag", acc.Name, fieldBase+"Writable"); err != nil {
						return err
					}
				}
				if acc.Signer {
					if err := validateRawIdent(acCtx+".signerFlag", acc.Name, fieldBase+"Signer"); err != nil {
						return err
					}
				}
				if acc.Optional {
					if err := validateRawIdent(acCtx+".optionalFlag", acc.Name, fieldBase+"Optional"); err != nil {
						return err
					}
				}
				param := formatAccountNameParam(acc.Name)
				if err := validateRawIdent(acCtx+".builderParam", acc.Name, param); err != nil {
					return err
				}
			case *idl.IdlInstructionAccounts:
				return fmt.Errorf("%s.accounts[%d]: composite account groups are not supported", ctx, ai)
			default:
				return fmt.Errorf("%s.accounts[%d]: unknown account item type %T", ctx, ai, accItem)
			}
		}
	}
	for ti, def := range i.Types {
		ctx := fmt.Sprintf("types[%d](name=%q)", ti, def.Name)
		if err := validatePascalIdent(ctx, def.Name); err != nil {
			return err
		}
		if err := validateTypeDefTy(ctx, def.Name, def.Ty); err != nil {
			return err
		}
	}
	return nil
}

func instructionStructTypeName(instructionName string) string {
	lower := strings.ToLower(instructionName)
	if strings.HasSuffix(lower, "instruction") {
		return tools.ToCamelUpper(instructionName)
	}
	return tools.ToCamelUpper(instructionName) + "Instruction"
}

func quoteIDL(s string) string {
	return fmt.Sprintf("%q", s)
}

func validateTypeDefTy(ctx, typeName string, ty idl.IdlTypeDefTy) error {
	if ty == nil {
		return fmt.Errorf("%s: type definition has nil type body", ctx)
	}
	switch vv := ty.(type) {
	case *idl.IdlTypeDefTyStruct:
		fields := vv.Fields
		if fields == nil {
			return nil
		}
		switch f := fields.(type) {
		case idl.IdlDefinedFieldsNamed:
			for fi, field := range f {
				fctx := fmt.Sprintf("%s.fields[%d](name=%q)", ctx, fi, field.Name)
				if err := validatePascalIdent(fctx, field.Name); err != nil {
					return err
				}
			}
		case idl.IdlDefinedFieldsTuple:
			_ = f
		}
	case *idl.IdlTypeDefTyEnum:
		enumExported := tools.ToCamelUpper(typeName)
		if vv.Variants.IsAllSimple() {
			for vi, variant := range vv.Variants {
				vctx := fmt.Sprintf("%s.variants[%d](name=%q)", ctx, vi, variant.Name)
				if err := validatePascalIdent(vctx, variant.Name); err != nil {
					return err
				}
				combo := formatSimpleEnumVariantName(variant.Name, enumExported)
				if err := validateRawIdent(vctx+".simpleEnumConst", variant.Name, combo); err != nil {
					return err
				}
			}
		} else {
			for vi, variant := range vv.Variants {
				vctx := fmt.Sprintf("%s.variants[%d](name=%q)", ctx, vi, variant.Name)
				if err := validatePascalIdent(vctx, variant.Name); err != nil {
					return err
				}
				vt := formatComplexEnumVariantTypeName(enumExported, variant.Name)
				if err := validateRawIdent(vctx+".complexVariantType", variant.Name, vt); err != nil {
					return err
				}
				if !variant.Fields.IsSome() {
					continue
				}
				switch df := variant.Fields.Unwrap().(type) {
				case idl.IdlDefinedFieldsNamed:
					for fi, field := range df {
						fctx := fmt.Sprintf("%s.fields[%d](name=%q)", vctx, fi, field.Name)
						if err := validatePascalIdent(fctx, field.Name); err != nil {
							return err
						}
					}
				case idl.IdlDefinedFieldsTuple:
				}
			}
		}
	default:
		return fmt.Errorf("%s: unsupported IDL type definition shape %T", ctx, ty)
	}
	return nil
}

func validatePascalIdent(context, raw string) error {
	ident := tools.ToCamelUpper(raw)
	return validateRawIdent(context, raw, ident)
}

func validateRawIdent(context, idlSource, goIdent string) error {
	if goIdent == "" {
		return fmt.Errorf("%s: empty Go identifier derived from IDL name %q", context, idlSource)
	}
	if !tools.IsValidIdent(goIdent) {
		return fmt.Errorf("%s: IDL name %q yields invalid Go identifier %q (must be a valid Go identifier for generated bindings)", context, idlSource, goIdent)
	}
	if tools.IsReservedKeyword(goIdent) {
		return fmt.Errorf("%s: IDL name %q yields Go reserved keyword %q", context, idlSource, goIdent)
	}
	return nil
}
