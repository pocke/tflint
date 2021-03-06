package evaluator

import (
	"reflect"

	"github.com/hashicorp/hcl"
	hclast "github.com/hashicorp/hcl/hcl/ast"
	hilast "github.com/hashicorp/hil/ast"
)

type hclVariable struct {
	Name         string `hcl:",key"`
	Default      interface{}
	Description  string
	DeclaredType string   `hcl:"type"`
	Fields       []string `hcl:",decodedFields"`
}

func detectVariables(listMap map[string]*hclast.ObjectList) (map[string]hilast.Variable, error) {
	varMap := make(map[string]hilast.Variable)

	for _, list := range listMap {
		var variables []*hclVariable
		if err := hcl.DecodeObject(&variables, list.Filter("variable")); err != nil {
			return nil, err
		}

		for _, v := range variables {
			if v.Default == nil {
				continue
			}
			varName := "var." + v.Name
			varMap[varName] = parseVariable(v.Default, v.DeclaredType)
		}
	}

	return varMap, nil
}

func parseVariable(val interface{}, varType string) hilast.Variable {
	// varType is overwrite invariably. Because, happen panic when used in incorrect type
	switch reflect.TypeOf(val).Kind() {
	case reflect.String:
		varType = "string"
	case reflect.Slice:
		varType = "list"
	case reflect.Map:
		varType = "map"
	default:
		varType = "string"
	}

	var hilVar hilast.Variable
	switch varType {
	case "string":
		hilVar = hilast.Variable{
			Type:  hilast.TypeString,
			Value: val,
		}
	case "map":
		// When HCL map var convert(parse) to Go var,
		// get map in slice. following example:
		//
		// ```HCL
		// key = {
		//     name = "test"
		//     value = "hcl"
		// }
		// ```
		//
		// Incorrect:
		//
		// map[string]string{
		//     "key": map[string][string]{
		//         "name":  "test",
		//         "value": "hcl",
		//     },
		// }
		//
		// Correct:
		//
		// []map[string]string{
		//     map[string]string{
		//         "name":  "test",
		//         "value": "hcl",
		//     },
		// }
		//
		fallthrough
	case "list":
		s := reflect.ValueOf(val)

		switch reflect.TypeOf(s.Index(0).Interface()).Kind() {
		case reflect.Map:
			var variables map[string]hilast.Variable
			variables = map[string]hilast.Variable{}
			for i := 0; i < s.Len(); i++ {
				ms := reflect.ValueOf(s.Index(i).Interface())
				for _, k := range ms.MapKeys() {
					key := k.Interface().(string)
					value := ms.MapIndex(reflect.ValueOf(key)).Interface()
					variables[key] = parseVariable(value, "")
				}
			}
			hilVar = hilast.Variable{
				Type:  hilast.TypeMap,
				Value: variables,
			}
		default:
			var variables []hilast.Variable
			for i := 0; i < s.Len(); i++ {
				variables = append(variables, parseVariable(s.Index(i).Interface(), ""))
			}
			hilVar = hilast.Variable{
				Type:  hilast.TypeList,
				Value: variables,
			}
		}
	}

	return hilVar
}
