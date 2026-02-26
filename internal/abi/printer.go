// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// FormatText returns a human-readable text representation of the contract spec.
func FormatText(spec *ContractSpec) string {
	var b strings.Builder

	if len(spec.Functions) > 0 {
		fmt.Fprintf(&b, "Functions (%d):\n", len(spec.Functions))
		for _, fn := range spec.Functions {
			fmt.Fprintf(&b, "  %s\n", formatFunction(fn))
		}
	}

	if len(spec.Structs) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Structs (%d):\n", len(spec.Structs))
		for _, s := range spec.Structs {
			fmt.Fprintf(&b, "  %s\n", s.Name)
			for _, f := range s.Fields {
				fmt.Fprintf(&b, "    %s: %s\n", f.Name, FormatTypeDef(f.Type))
			}
		}
	}

	if len(spec.Enums) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Enums (%d):\n", len(spec.Enums))
		for _, e := range spec.Enums {
			fmt.Fprintf(&b, "  %s\n", e.Name)
			for _, c := range e.Cases {
				fmt.Fprintf(&b, "    %s = %d\n", c.Name, c.Value)
			}
		}
	}

	if len(spec.Unions) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Unions (%d):\n", len(spec.Unions))
		for _, u := range spec.Unions {
			fmt.Fprintf(&b, "  %s\n", u.Name)
			for _, c := range u.Cases {
				switch c.Kind {
				case xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseVoidV0:
					fmt.Fprintf(&b, "    %s\n", c.VoidCase.Name)
				case xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseTupleV0:
					types := make([]string, len(c.TupleCase.Type))
					for i, t := range c.TupleCase.Type {
						types[i] = FormatTypeDef(t)
					}
					fmt.Fprintf(&b, "    %s(%s)\n", c.TupleCase.Name, strings.Join(types, ", "))
				}
			}
		}
	}

	if len(spec.ErrorEnums) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Error Enums (%d):\n", len(spec.ErrorEnums))
		for _, e := range spec.ErrorEnums {
			fmt.Fprintf(&b, "  %s\n", e.Name)
			for _, c := range e.Cases {
				fmt.Fprintf(&b, "    %s = %d\n", c.Name, c.Value)
			}
		}
	}

	if len(spec.Events) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Events (%d):\n", len(spec.Events))
		for _, ev := range spec.Events {
			fmt.Fprintf(&b, "  %s\n", string(ev.Name))
			for _, p := range ev.Params {
				loc := "data"
				if p.Location == xdr.ScSpecEventParamLocationV0ScSpecEventParamLocationTopicList {
					loc = "topic"
				}
				fmt.Fprintf(&b, "    %s: %s (%s)\n", p.Name, FormatTypeDef(p.Type), loc)
			}
		}
	}

	return b.String()
}

// jsonSpec is the JSON-friendly representation of a contract spec.
type jsonSpec struct {
	Functions  []jsonFunction `json:"functions,omitempty"`
	Structs    []jsonStruct   `json:"structs,omitempty"`
	Enums      []jsonEnum     `json:"enums,omitempty"`
	Unions     []jsonUnion    `json:"unions,omitempty"`
	ErrorEnums []jsonEnum     `json:"error_enums,omitempty"`
	Events     []jsonEvent    `json:"events,omitempty"`
}

type jsonFunction struct {
	Name    string      `json:"name"`
	Inputs  []jsonField `json:"inputs,omitempty"`
	Outputs []string    `json:"outputs,omitempty"`
	Doc     string      `json:"doc,omitempty"`
}

type jsonStruct struct {
	Name   string      `json:"name"`
	Fields []jsonField `json:"fields,omitempty"`
	Doc    string      `json:"doc,omitempty"`
}

type jsonField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type jsonEnum struct {
	Name  string         `json:"name"`
	Cases []jsonEnumCase `json:"cases,omitempty"`
	Doc   string         `json:"doc,omitempty"`
}

type jsonEnumCase struct {
	Name  string `json:"name"`
	Value uint32 `json:"value"`
}

type jsonUnion struct {
	Name  string          `json:"name"`
	Cases []jsonUnionCase `json:"cases,omitempty"`
	Doc   string          `json:"doc,omitempty"`
}

type jsonUnionCase struct {
	Name  string   `json:"name"`
	Types []string `json:"types,omitempty"`
}

type jsonEvent struct {
	Name   string           `json:"name"`
	Params []jsonEventParam `json:"params,omitempty"`
	Doc    string           `json:"doc,omitempty"`
}

type jsonEventParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location"`
}

// FormatJSON returns a JSON representation of the contract spec with resolved
// type names.
func FormatJSON(spec *ContractSpec) (string, error) {
	js := jsonSpec{}

	for _, fn := range spec.Functions {
		jf := jsonFunction{
			Name: string(fn.Name),
			Doc:  fn.Doc,
		}
		for _, inp := range fn.Inputs {
			jf.Inputs = append(jf.Inputs, jsonField{Name: inp.Name, Type: FormatTypeDef(inp.Type)})
		}
		for _, out := range fn.Outputs {
			jf.Outputs = append(jf.Outputs, FormatTypeDef(out))
		}
		js.Functions = append(js.Functions, jf)
	}

	for _, s := range spec.Structs {
		jst := jsonStruct{Name: string(s.Name), Doc: s.Doc}
		for _, f := range s.Fields {
			jst.Fields = append(jst.Fields, jsonField{Name: f.Name, Type: FormatTypeDef(f.Type)})
		}
		js.Structs = append(js.Structs, jst)
	}

	for _, e := range spec.Enums {
		je := jsonEnum{Name: string(e.Name), Doc: e.Doc}
		for _, c := range e.Cases {
			je.Cases = append(je.Cases, jsonEnumCase{Name: c.Name, Value: uint32(c.Value)})
		}
		js.Enums = append(js.Enums, je)
	}

	for _, u := range spec.Unions {
		ju := jsonUnion{Name: string(u.Name), Doc: u.Doc}
		for _, c := range u.Cases {
			switch c.Kind {
			case xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseVoidV0:
				ju.Cases = append(ju.Cases, jsonUnionCase{Name: c.VoidCase.Name})
			case xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseTupleV0:
				types := make([]string, len(c.TupleCase.Type))
				for i, t := range c.TupleCase.Type {
					types[i] = FormatTypeDef(t)
				}
				ju.Cases = append(ju.Cases, jsonUnionCase{Name: c.TupleCase.Name, Types: types})
			}
		}
		js.Unions = append(js.Unions, ju)
	}

	for _, e := range spec.ErrorEnums {
		je := jsonEnum{Name: string(e.Name), Doc: e.Doc}
		for _, c := range e.Cases {
			je.Cases = append(je.Cases, jsonEnumCase{Name: c.Name, Value: uint32(c.Value)})
		}
		js.ErrorEnums = append(js.ErrorEnums, je)
	}

	for _, ev := range spec.Events {
		jev := jsonEvent{Name: string(ev.Name), Doc: ev.Doc}
		for _, p := range ev.Params {
			loc := "data"
			if p.Location == xdr.ScSpecEventParamLocationV0ScSpecEventParamLocationTopicList {
				loc = "topic"
			}
			jev.Params = append(jev.Params, jsonEventParam{
				Name:     p.Name,
				Type:     FormatTypeDef(p.Type),
				Location: loc,
			})
		}
		js.Events = append(js.Events, jev)
	}

	out, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling JSON: %w", err)
	}
	return string(out), nil
}

func formatFunction(fn xdr.ScSpecFunctionV0) string {
	params := make([]string, len(fn.Inputs))
	for i, inp := range fn.Inputs {
		params[i] = fmt.Sprintf("%s: %s", inp.Name, FormatTypeDef(inp.Type))
	}

	ret := "Void"
	if len(fn.Outputs) > 0 {
		ret = FormatTypeDef(fn.Outputs[0])
	}

	return fmt.Sprintf("%s(%s) -> %s", string(fn.Name), strings.Join(params, ", "), ret)
}

// FormatTypeDef returns a human-readable string for an ScSpecTypeDef.
func FormatTypeDef(td xdr.ScSpecTypeDef) string {
	switch td.Type {
	case xdr.ScSpecTypeScSpecTypeVal:
		return "Val"
	case xdr.ScSpecTypeScSpecTypeBool:
		return "Bool"
	case xdr.ScSpecTypeScSpecTypeVoid:
		return "Void"
	case xdr.ScSpecTypeScSpecTypeError:
		return "Error"
	case xdr.ScSpecTypeScSpecTypeU32:
		return "U32"
	case xdr.ScSpecTypeScSpecTypeI32:
		return "I32"
	case xdr.ScSpecTypeScSpecTypeU64:
		return "U64"
	case xdr.ScSpecTypeScSpecTypeI64:
		return "I64"
	case xdr.ScSpecTypeScSpecTypeTimepoint:
		return "Timepoint"
	case xdr.ScSpecTypeScSpecTypeDuration:
		return "Duration"
	case xdr.ScSpecTypeScSpecTypeU128:
		return "U128"
	case xdr.ScSpecTypeScSpecTypeI128:
		return "I128"
	case xdr.ScSpecTypeScSpecTypeU256:
		return "U256"
	case xdr.ScSpecTypeScSpecTypeI256:
		return "I256"
	case xdr.ScSpecTypeScSpecTypeBytes:
		return "Bytes"
	case xdr.ScSpecTypeScSpecTypeString:
		return "String"
	case xdr.ScSpecTypeScSpecTypeSymbol:
		return "Symbol"
	case xdr.ScSpecTypeScSpecTypeAddress:
		return "Address"
	case xdr.ScSpecTypeScSpecTypeMuxedAddress:
		return "MuxedAddress"
	case xdr.ScSpecTypeScSpecTypeOption:
		if td.Option != nil {
			return fmt.Sprintf("Option<%s>", FormatTypeDef(td.Option.ValueType))
		}
		return "Option<?>"
	case xdr.ScSpecTypeScSpecTypeResult:
		if td.Result != nil {
			return fmt.Sprintf("Result<%s, %s>", FormatTypeDef(td.Result.OkType), FormatTypeDef(td.Result.ErrorType))
		}
		return "Result<?, ?>"
	case xdr.ScSpecTypeScSpecTypeVec:
		if td.Vec != nil {
			return fmt.Sprintf("Vec<%s>", FormatTypeDef(td.Vec.ElementType))
		}
		return "Vec<?>"
	case xdr.ScSpecTypeScSpecTypeMap:
		if td.Map != nil {
			return fmt.Sprintf("Map<%s, %s>", FormatTypeDef(td.Map.KeyType), FormatTypeDef(td.Map.ValueType))
		}
		return "Map<?, ?>"
	case xdr.ScSpecTypeScSpecTypeTuple:
		if td.Tuple != nil {
			types := make([]string, len(td.Tuple.ValueTypes))
			for i, t := range td.Tuple.ValueTypes {
				types[i] = FormatTypeDef(t)
			}
			return fmt.Sprintf("(%s)", strings.Join(types, ", "))
		}
		return "()"
	case xdr.ScSpecTypeScSpecTypeBytesN:
		if td.BytesN != nil {
			return fmt.Sprintf("BytesN(%d)", td.BytesN.N)
		}
		return "BytesN(?)"
	case xdr.ScSpecTypeScSpecTypeUdt:
		if td.Udt != nil {
			return td.Udt.Name
		}
		return "UDT(?)"
	default:
		return fmt.Sprintf("Unknown(%d)", td.Type)
	}
}
