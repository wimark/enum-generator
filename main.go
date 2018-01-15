package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type EnumVariantMap map[string]string

type EnumInfo struct {
	Constraint *string        `toml:"constraint"`
	Default    *string        `toml:"default"`
	Variants   EnumVariantMap `toml:variants`
}
type EnumTypeMap map[string]EnumInfo

const HEADER string = `
package %pkgname
import (
	%packages
	"errors"
)`
const ENUM_TYPENAME string = `
type %type string`
const ENUM_VALUES string = `
const %type%name %type = "%value"`
const ENUM_GETPTR string = `
func (self %type) GetPtr() *%type { var v = self; return &v; }`
const ENUM_STRING_HEADER string = `
func (self *%type) String() string {
	switch *self {`
const ENUM_STRING_CASE string = `
	case %type%name:
		return "%value"`
const ENUM_STRING_DEFAULT string = `
	if len(*self) == 0 { return "%value" }`
const ENUM_STRING_FOOTER string = `
	}%defaultcode
	panic(errors.New("Invalid value of %type"))
}`
const ENUM_MARSHAL_HEADER string = `
func (self *%type) MarshalJSON() ([]byte, error) {
	switch *self {`
const ENUM_MARSHAL_CASE string = `
	case %type%name:
		return json.Marshal("%value")`
const ENUM_MARSHAL_DEFAULT string = `
	if len(*self) == 0 { return json.Marshal("%value") }`
const ENUM_GETTER_FOOTER string = `
	}%defaultcode
	return nil, errors.New("Invalid value of %type")
}`
const ENUM_GETBSON_HEADER string = `
func (self *%type) GetBSON() (interface{}, error) {
	switch *self {`
const ENUM_GETBSON_CASE string = `
	case %type%name:
		return "%value", nil`
const ENUM_GETBSON_DEFAULT string = `
	if len(*self) == 0 { return "%value", nil }`
const ENUM_UNMARSHAL_HEADER string = `
func (self *%type) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch s {`
const ENUM_SETTER_CASE string = `
	case "%value":
		*self = %type%name
		return nil`
const ENUM_SETTER_DEFAULT string = `
	if len(s) == 0 {
		*self = %type%name
		return nil
	}`
const ENUM_SETTER_FOOTER string = `
	}%defaultcode
	return errors.New("Unknown %type")
}`
const ENUM_SETBSON_HEADER string = `
func (self *%type) SetBSON(v bson.Raw) error {
	var s string
	if err := v.Unmarshal(&s); err != nil {
		return err
	}
	switch s {`
const ASSOC_TYPE string = `
type %type struct {
	Type %constr "json:\"type\""
	Data interface{} "json:\"data\""
}`
const ASSOC_UNMARSHAL_HEADER string = `
func (self *%type) UnmarshalJSON(b []byte) error {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if doc == nil {
		return nil
	}
	var t_raw, t_found = doc["type"]
	if !t_found {
		return nil
	}
	var data_raw, data_found = doc["data"]
	if bytes.Equal(data_raw, []byte("null")) {
		data_found = false
	}
	var t %constr
	if t_err := json.Unmarshal(t_raw, &t); t_err != nil {
		return t_err
	}
	switch t {`
const ASSOC_UNMARSHAL_CASE string = `
	case %constr%name:
		if !data_found {
			return errors.New("No associated data found for enum %type")
		}
		var d %value
		var data_err = json.Unmarshal(data_raw, &d)
		if data_err != nil {
			return data_err
		}
		self.Data = &d`
const ASSOC_SETTER_CASE_NULL string = `
	case %constr%name:
		break`
const ASSOC_SETTER_FOOTER string = `
	}
	self.Type = t
	return nil
}`
const ASSOC_SETBSON_HEADER string = `
func (self *%type) SetBSON(v bson.Raw) error {
	var in = map[string]bson.Raw{}
	if err := v.Unmarshal(&in); err != nil {
		return err
	}
	if in == nil {
		return nil
	}
	var t_raw, t_found = in["type"]
	if !t_found {
		return nil
	}
	var data_raw, data_found = in["data"]
	if bytes.Equal(data_raw.Data, []byte("null")) {
		data_found = false
	}
	var t %constr
	if t_err := t_raw.Unmarshal(&t); t_err != nil {
		return t_err
	}
	switch t {`
const ASSOC_SETBSON_CASE string = `
	case %constr%name:
		if !data_found {
			return errors.New("No associated data found for enum %type")
		}
		var d %value
		var data_err = data_raw.Unmarshal(&d)
		if data_err != nil {
			return data_err
		}
		self.Data = &d`

type ReplaceMap map[string]string
type TemplateMap map[string]string

func replacer(origin string, replaces ReplaceMap) string {

	var result = origin
	for key, value := range replaces {
		result = strings.Replace(result, key, value, -1)
	}
	return result
}

func generateHeader(pkgname string, enable_json, enable_bson, has_assoc bool) string {
	var packages = []string{}
	if enable_json {
		packages = append(packages, `"encoding/json"`)
	}
	if enable_bson {
		packages = append(packages, `"gopkg.in/mgo.v2/bson"`)
	}
	if has_assoc && (enable_bson || enable_json) {
		packages = append(packages, `"bytes"`)
	}
	return replacer(HEADER,
		ReplaceMap{"%packages": strings.Join(packages, "\n"), "%pkgname": pkgname})

}

func generateEnumTypeName(name string) string {
	return replacer(ENUM_TYPENAME,
		ReplaceMap{"%type": name})
}
func generateEnumValues(etype, name, value string) string {
	return replacer(ENUM_VALUES,
		ReplaceMap{"%name": name, "%type": etype, "%value": value})
}
func generateEnumGetPtr(name string) string {
	return replacer(ENUM_GETPTR,
		ReplaceMap{"%type": name})
}
func generateEnumFunction(name string, values sort.StringSlice, enum_info EnumInfo,
	templates TemplateMap) string {

	var code string
	code += replacer(templates["header"],
		ReplaceMap{"%type": name})

	for _, var_name := range values {
		var value = enum_info.Variants[var_name]
		code += replacer(templates["case"],
			ReplaceMap{"%name": var_name, "%type": name, "%value": value})
	}
	var defaultCode = ""
	if enum_info.Default != nil {
		defaultCode = replacer(templates["default"],
			ReplaceMap{"%name": *enum_info.Default, "%type": name, "%value": enum_info.Variants[*enum_info.Default]})
	}
	code += replacer(templates["footer"],
		ReplaceMap{"%type": name, "%defaultcode": defaultCode})

	return code
}
func generateEnumType(enum_name string, enum_info EnumInfo, enable_json bool, enable_bson bool) string {
	var code string
	var sorted_var_list = sort.StringSlice{}
	for v := range enum_info.Variants {
		sorted_var_list = append(sorted_var_list, v)
	}
	sorted_var_list.Sort()

	code += generateEnumTypeName(enum_name)
	for _, var_name := range sorted_var_list {
		code += generateEnumValues(enum_name, var_name, enum_info.Variants[var_name])
	}
	code += generateEnumGetPtr(enum_name)
	code += generateEnumFunction(enum_name, sorted_var_list, enum_info,
		TemplateMap{
			"header":  ENUM_STRING_HEADER,
			"footer":  ENUM_STRING_FOOTER,
			"default": ENUM_STRING_DEFAULT,
			"case":    ENUM_STRING_CASE,
		})
	if enable_json {
		code += generateEnumFunction(enum_name, sorted_var_list, enum_info,
			TemplateMap{
				"header":  ENUM_MARSHAL_HEADER,
				"footer":  ENUM_GETTER_FOOTER,
				"default": ENUM_MARSHAL_DEFAULT,
				"case":    ENUM_MARSHAL_CASE,
			})
	}
	if enable_bson {
		code += generateEnumFunction(enum_name, sorted_var_list, enum_info,
			TemplateMap{
				"header":  ENUM_GETBSON_HEADER,
				"footer":  ENUM_GETTER_FOOTER,
				"default": ENUM_GETBSON_DEFAULT,
				"case":    ENUM_GETBSON_CASE,
			})
	}
	if enable_json {
		code += generateEnumFunction(enum_name, sorted_var_list, enum_info,
			TemplateMap{
				"header":  ENUM_UNMARSHAL_HEADER,
				"footer":  ENUM_SETTER_FOOTER,
				"default": ENUM_SETTER_DEFAULT,
				"case":    ENUM_SETTER_CASE,
			})
	}
	if enable_bson {
		code += generateEnumFunction(enum_name, sorted_var_list, enum_info,
			TemplateMap{
				"header":  ENUM_SETBSON_HEADER,
				"footer":  ENUM_SETTER_FOOTER,
				"default": ENUM_SETTER_DEFAULT,
				"case":    ENUM_SETTER_CASE,
			})
	}

	return code
}

func generateAssocEnumTypeName(name, constr string) string {
	return replacer(ASSOC_TYPE,
		ReplaceMap{"%type": name, "%constr": constr})
}
func generateAssocEnumFunction(name string, values sort.StringSlice, enum_info EnumInfo,
	templates TemplateMap) string {

	var code string
	code += replacer(templates["header"],
		ReplaceMap{"%type": name, "%constr": *enum_info.Constraint})

	for _, var_name := range values {
		var value = enum_info.Variants[var_name]
		if value == "null" {
			code += replacer(templates["case_null"],
				ReplaceMap{"%name": var_name, "%type": name, "%value": value, "%constr": *enum_info.Constraint})
		} else {
			code += replacer(templates["case"],
				ReplaceMap{"%name": var_name, "%type": name, "%value": value, "%constr": *enum_info.Constraint})
		}
	}
	code += replacer(templates["footer"],
		ReplaceMap{"%type": name})

	return code
}

func generateAssocEnumType(enum_name string, enum_info EnumInfo, enable_json bool, enable_bson bool) string {

	var sorted_var_list = sort.StringSlice{}
	for v := range enum_info.Variants {
		sorted_var_list = append(sorted_var_list, string(v))
	}
	sorted_var_list.Sort()

	var code string

	code += generateAssocEnumTypeName(enum_name, *enum_info.Constraint)

	if enable_json {
		code += generateAssocEnumFunction(enum_name, sorted_var_list, enum_info,
			TemplateMap{
				"header":    ASSOC_UNMARSHAL_HEADER,
				"footer":    ASSOC_SETTER_FOOTER,
				"case_null": ASSOC_SETTER_CASE_NULL,
				"case":      ASSOC_UNMARSHAL_CASE,
			})
	}
	if enable_bson {
		code += generateAssocEnumFunction(enum_name, sorted_var_list, enum_info,
			TemplateMap{
				"header":    ASSOC_SETBSON_HEADER,
				"footer":    ASSOC_SETTER_FOOTER,
				"case_null": ASSOC_SETTER_CASE_NULL,
				"case":      ASSOC_SETBSON_CASE,
			})
	}

	return code
}

func generateEnumCode(m EnumTypeMap, pkg string, enable_json bool, enable_bson bool) string {
	var code string
	var has_assoc = false
	var sorted_list = sort.StringSlice{}
	for enum_name, enum_val := range m {
		sorted_list = append(sorted_list, enum_name)
		if enum_val.Constraint != nil {
			has_assoc = true
		}
	}
	sorted_list.Sort()

	code += generateHeader(pkg, enable_json, enable_bson, has_assoc)

	for _, enum_name := range sorted_list {
		if m[enum_name].Constraint == nil {
			code += generateEnumType(enum_name, m[enum_name], enable_json, enable_bson)
		} else {
			code += generateAssocEnumType(enum_name, m[enum_name], enable_json, enable_bson)
		}
	}

	return code
}

func main() {
	var enable_json bool = false
	flag.BoolVar(&enable_json, "enable-json", false,
		"Enable generation of JSON code")

	var enable_bson bool = false
	flag.BoolVar(&enable_bson, "enable-bson", false,
		"Enable generation of BSON code")

	var package_name string
	flag.StringVar(&package_name, "package", "main",
		"Go package name")

	flag.Parse()

	var data string
	var scanner = bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		data += scanner.Text()
		data += "\n"
	}

	var m = EnumTypeMap{}
	var err = toml.Unmarshal([]byte(data), &m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}
	var result = generateEnumCode(m, package_name, enable_json, enable_bson)
	// beautify a bit
	result = strings.Replace(result, "\ntype", "\n\ntype", -1)
	result = strings.Replace(result, "\nfunc", "\n\nfunc", -1)
	fmt.Println(result)
}
