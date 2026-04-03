//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
)

func (g *Generator) isComplexEnum(envel idltype.IdlType) bool {
	switch vv := envel.(type) {
	case *idltype.Defined:
		_, ok := g.complexEnumRegistry[vv.Name]
		return ok
	}
	return false
}

func (g *Generator) registerComplexEnumType(name string) {
	g.complexEnumRegistry[name] = struct{}{}
}

func (g *Generator) registerComplexEnums(def idl.IdlTypeDef) {
	switch vv := def.Ty.(type) {
	case *idl.IdlTypeDefTyEnum:
		enumTypeName := def.Name
		if !vv.IsAllSimple() {
			g.registerComplexEnumType(enumTypeName)
		}
	case idl.IdlTypeDefTyEnum:
		enumTypeName := def.Name
		if !vv.IsAllSimple() {
			g.registerComplexEnumType(enumTypeName)
		}
	}
}
