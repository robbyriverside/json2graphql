package json2graphql

import (
	"encoding/json"
	"fmt"
	"github.com/graphql-go/graphql"
	"io/ioutil"
	"strings"
)

const (
	Unknown = iota
	ResolveConflict
	InputTypeConflict
	QueryConflict
	MutationConflict
	InternalError
	InvalidSchema
	InvalidMap
)

type maker struct {
	TypeMap  InputMap
	FnMap    ResolveMap
	query    map[string]string
	mutation map[string]string
	Errors   []*j2gError
}

func NewMaker() *maker {
	return &maker{
		TypeMap:  make(InputMap),
		FnMap:    make(ResolveMap),
		query:    make(map[string]string),
		mutation: make(map[string]string),
		Errors:   make([]*j2gError, 0),
	}
}

type j2gError struct {
	Code    int
	Message string
}

func (e j2gError) Error() string {
	return e.Message
}

func J2GErrorf(code int, format string, args ...interface{}) *j2gError {
	return &j2gError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

func (m maker) ErrorString() string {
	result := ""
	for _, err := range m.Errors {
		result += err.Message + "\n"
	}
	return result
}

func (m *maker) J2GErrorf(code int, format string, args ...interface{}) *j2gError {
	err := J2GErrorf(code, format, args...)
	m.Errors = append(m.Errors, err)
	return err
}

type ResolveMap map[string]graphql.FieldResolveFn
type InputMap map[string]graphql.Input

func (m *maker) ResolveFn(name string, fn graphql.FieldResolveFn) {
	if _, ok := m.FnMap[name]; ok {
		m.J2GErrorf(ResolveConflict, "resolve name %q already d!", name)
	}
	m.FnMap[name] = fn
}

func (m *maker) InputType(name string, in graphql.Input) {
	if _, ok := m.TypeMap[name]; ok {
		m.J2GErrorf(InputTypeConflict, "input type name %q already d!", name)
	}
	m.TypeMap[name] = in
}

func (m *maker) Resolve(fnmap ResolveMap) {
	for name, fn := range fnmap {
		m.ResolveFn(name, fn)
	}
}

func (m *maker) Input(inmap InputMap) {
	for name, in := range inmap {
		m.InputType(name, in)
	}
}

func (m *maker) Query(name, json string, maps ...interface{}) {
	if _, ok := m.query[name]; ok {
		m.J2GErrorf(QueryConflict, "query type name %q already d!", name)
	}
	m.query[name] = json
	for _, mp := range maps {
		switch xmap := mp.(type) {
		case ResolveMap:
			m.Resolve(xmap)
		case InputMap:
			m.Input(xmap)
		default:
			m.J2GErrorf(InvalidMap, "unknown map for query %q value: %#v", name, xmap)
		}
	}
}

func (m *maker) Mutation(name, json string, maps ...interface{}) {
	if _, ok := m.mutation[name]; ok {
		m.J2GErrorf(MutationConflict, "mutation type name %q already d!", name)
	}
	m.mutation[name] = json
	for _, mp := range maps {
		switch xmap := mp.(type) {
		case ResolveMap:
			m.Resolve(xmap)
		case InputMap:
			m.Input(xmap)
		default:
			m.J2GErrorf(InvalidMap, "unknown map for mutation %q value: %#v", name, xmap)
		}
	}
}

func (m maker) MakeSchema() (*graphql.Schema, error) {
	if len(m.Errors) > 0 {
		return nil, J2GErrorf(InvalidSchema, "schema contains errors: %s", m.ErrorString())
	}
	if len(m.query) == 0 {
		return nil, J2GErrorf(InvalidSchema, "shema contains no queries, abort")
	}
	spec := newGQLSpec(&m)
	for name, value := range m.query {
		field := &gqlField{}
		json.Unmarshal([]byte(value), field)
		spec.Query[name] = field
	}
	for name, value := range m.mutation {
		field := &gqlField{}
		json.Unmarshal([]byte(value), field)
		spec.Mutation[name] = field
	}
	return spec.MakeSchema()
}

func (m maker) LoadSchema(filename string) (*graphql.Schema, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, J2GErrorf(InternalError, "unable to read schema file %q: %s", filename, err)
	}

	spec := newGQLSpec(&m)
	err = json.Unmarshal(file, &spec)
	if err != nil {
		return nil, fmt.Errorf("json error: %s", err)
	}
	return spec.MakeSchema()
}

type gqlArg struct {
	Type string
}

type gqlArgmap map[string]*gqlArg

type gqlField struct {
	Args    gqlArgmap
	Type    string
	Resolve string
}

type gqlObject map[string]*gqlField

type gqlSpec struct {
	Mutation gqlObject
	Query    gqlObject
	Parser   *maker
}

func newGQLSpec(p *maker) *gqlSpec {
	return &gqlSpec{
		Mutation: make(gqlObject),
		Query:    make(gqlObject),
		Parser:   p,
	}
}

func (m maker) internInput(value string) (graphql.Input, error) {
	nonnull := false
	list := strings.HasPrefix(value, "[")
	if list && strings.HasPrefix(value, "]") {
		value = value[1 : len(value)-1]
	} else if list {
		nonnull = strings.HasSuffix(value, "!")
		if nonnull {
			value = value[:len(value)-1]
			if list && strings.HasPrefix(value, "]") {
				value = value[1 : len(value)-1]
			}
		} else {
			value = value[1:]
		}
	}
	required := strings.HasSuffix(value, "!")
	if required {
		value = value[:len(value)-1]
	}
	var result graphql.Input
	switch value {
	case "String":
		result = graphql.String
	case "Int":
		result = graphql.Int
	case "Float":
		result = graphql.Float
	case "Boolean":
		result = graphql.Boolean
	case "ID":
		result = graphql.ID
	default:
		if _, ok := m.TypeMap[value]; !ok {
			return nil, fmt.Errorf("invalid input type: %s", value)
		}
		result = m.TypeMap[value]
	}
	if required {
		result = graphql.NewNonNull(result)
	}
	if list {
		result = graphql.NewList(result)
		if nonnull {
			result = graphql.NewNonNull(result)
		}
	}

	return result, nil
}

func (m maker) internResolveFn(name string) (graphql.FieldResolveFn, error) {
	fn, ok := m.FnMap[name]
	if !ok {
		return nil, fmt.Errorf("unknown function: %s", name)
	}
	return fn, nil
}

func (ctx *gqlSpec) MakeArg(spec *gqlArg) (*graphql.ArgumentConfig, error) {
	argType, err := ctx.Parser.internInput(spec.Type)
	if err != nil {
		return nil, err
	}
	arg := graphql.ArgumentConfig{
		Type: argType,
	}
	return &arg, nil
}

func (ctx *gqlSpec) MakeField(spec *gqlField) (*graphql.Field, error) {
	args := graphql.FieldConfigArgument{}
	for key, value := range spec.Args {
		arg, err := ctx.MakeArg(value)
		if err != nil {
			return nil, fmt.Errorf("bad arg %s: %s", key, err)
		}
		args[key] = arg
	}
	in, err := ctx.Parser.internInput(spec.Type)
	if err != nil {
		return nil, fmt.Errorf("field error bad type %s: %s", spec.Type, err)
	}
	fn, err := ctx.Parser.internResolveFn(spec.Resolve)
	if err != nil {
		return nil, fmt.Errorf("field error bad resolve %s: %s", spec.Resolve, err)
	}
	field := graphql.Field{
		Args:    args,
		Type:    in,
		Resolve: fn,
	}
	return &field, nil
}

func (ctx *gqlSpec) MakeObject(spec gqlObject, name string) (*graphql.Object, error) {
	fields := graphql.Fields{}
	for key, value := range spec {
		field, err := ctx.MakeField(value)
		if err != nil {
			return nil, fmt.Errorf("bad field %s: %s", key, err)
		}
		fields[key] = field
	}
	object := graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: fields,
	})
	return object, nil
}

func (spec gqlSpec) MakeSchema() (*graphql.Schema, error) {
	queryType, err := spec.MakeObject(spec.Query, "Query")
	if err != nil {
		return nil, fmt.Errorf("bad query: %s", err)
	}
	mutationType, err := spec.MakeObject(spec.Mutation, "Mutation")
	if err != nil {
		return nil, fmt.Errorf("bad mudation: %s", err)
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	return &schema, err
}
