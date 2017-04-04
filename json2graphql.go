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
	TypeConflict
	QueryConflict
	MutationConflict
	FieldConflict
	InternalError
	InvalidSchema
	InvalidMap
)

type FieldMap map[string]string
type ResolveMap map[string]graphql.FieldResolveFn

type maker struct {
	TypeMap  map[string]FieldMap
	FnMap    ResolveMap
	query    FieldMap
	mutation FieldMap
	Errors   []*j2gError
}

func NewMaker() *maker {
	return &maker{
		TypeMap:  make(map[string]FieldMap),
		FnMap:    make(ResolveMap),
		query:    make(FieldMap),
		mutation: make(FieldMap),
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

func (m *maker) Resolve(name string, fn graphql.FieldResolveFn) {
	if _, ok := m.FnMap[name]; ok {
		m.J2GErrorf(ResolveConflict, "resolve name %q already defined!", name)
	}
	m.FnMap[name] = fn
}

func (m *maker) ResolveMap(fnmap ResolveMap) {
	for name, fn := range fnmap {
		m.Resolve(name, fn)
	}
}

func (m *maker) Field(typename, name, json string, fnmaps ...ResolveMap) {
	fmap, ok := m.TypeMap[typename]
	if !ok {
		fmap = FieldMap{}
		m.TypeMap[typename] = fmap
	}
	if _, ok := fmap[name]; ok {
		m.J2GErrorf(FieldConflict, "field %s already defined in %s", name, typename)
	}
	fmap[name] = json
	for _, fnmap := range fnmaps {
		m.ResolveMap(fnmap)
	}
}

func (m *maker) FieldMap(typename string, fmap FieldMap) {
	for field, json := range fmap {
		m.Field(typename, field, json)
	}
}

func (m *maker) Object(typename string, fmap FieldMap, fnmap ResolveMap) {
	m.FieldMap(typename, fmap)
	m.ResolveMap(fnmap)
}

func (m *maker) Query(name, json string, fnmaps ...ResolveMap) {
	if _, ok := m.query[name]; ok {
		m.J2GErrorf(QueryConflict, "query type name %q already defined", name)
	}
	m.query[name] = json
	for _, fnmap := range fnmaps {
		m.ResolveMap(fnmap)
	}
}

func (m *maker) Mutation(name, json string, fnmaps ...ResolveMap) {
	if _, ok := m.mutation[name]; ok {
		m.J2GErrorf(MutationConflict, "mutation type name %q already defined", name)
	}
	m.mutation[name] = json
	for _, fnmap := range fnmaps {
		m.ResolveMap(fnmap)
	}
}

func (m maker) readFields(obj *gqlObject, fmap FieldMap) {
	for name, value := range fmap {
		field := &gqlField{}
		json.Unmarshal([]byte(value), field)
		obj.FieldMap[name] = field
	}
}

func (m maker) MakeSpec() (*gqlSpec, error) {
	if len(m.Errors) > 0 {
		return nil, J2GErrorf(InvalidSchema, "schema contains errors: %s", m.ErrorString())
	}
	if len(m.query) == 0 {
		return nil, J2GErrorf(InvalidSchema, "shema contains no queries, abort")
	}
	spec := newGQLSpec(&m)
	for typename, fmap := range m.TypeMap {
		obj := newGQLObject(typename)
		m.readFields(obj, fmap)
		spec.TypeMap[typename] = obj
	}
	m.readFields(spec.query, m.query)
	m.readFields(spec.mutation, m.mutation)
	return spec, nil
}

func (m maker) MakeSchema() (*graphql.Schema, error) {
	spec, err := m.MakeSpec()
	if err != nil {
		return nil, err
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

type gqlObject struct {
	Name     string
	FieldMap gqlFieldMap
}

func newGQLObject(name string) *gqlObject {
	return &gqlObject{
		Name:     name,
		FieldMap: make(gqlFieldMap),
	}
}

type gqlFieldMap map[string]*gqlField

type gqlObjectMap map[string]*gqlObject

type gqlSpec struct {
	TypeMap   gqlObjectMap
	mutation  *gqlObject
	query     *gqlObject
	Parser    *maker
	Loops     map[string]uint
	ObjectMap map[string]*graphql.Object
}

func newGQLSpec(p *maker) *gqlSpec {
	return &gqlSpec{
		TypeMap:   make(gqlObjectMap),
		mutation:  newGQLObject("mutation"),
		query:     newGQLObject("query"),
		Parser:    p,
		Loops:     make(map[string]uint),
		ObjectMap: make(map[string]*graphql.Object),
	}
}

func (spec *gqlSpec) getObject(typename string) (*graphql.Object, error) {
	object, ok := spec.ObjectMap[typename]
	if !ok {
		if _, ok := spec.Loops[typename]; ok {
			order := make([]string, len(spec.Loops))
			for id, i := range spec.Loops {
				order[i] = id
			}
			return nil, fmt.Errorf("type recursion detected: %s, %s", strings.Join(order, ", "), typename)
		}
		spec.Loops[typename] = uint(len(spec.Loops))
		defer func() { delete(spec.Loops, typename) }()
		ospec, found := spec.TypeMap[typename]
		if !found {
			return nil, fmt.Errorf("unknown object type: %s", typename)
		}
		var err error
		object, err = spec.MakeObject(ospec)
		if err != nil {
			return nil, fmt.Errorf("bad object %s: %s", typename, err)
		}
	}
	return object, nil
}

func (spec gqlSpec) internType(value string) (graphql.Input, error) {
	nonnull := false
	list := strings.HasPrefix(value, "[")
	if list && strings.HasSuffix(value, "]") {
		value = value[1 : len(value)-1]
	} else if list {
		nonnull = strings.HasSuffix(value, "!")
		if nonnull {
			value = value[:len(value)-1]
			if list && strings.HasSuffix(value, "]") {
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
	var err error
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
		result, err = spec.getObject(value)
		if err != nil {
			state := value
			if required {
				state = "nonnull " + state
			}
			if nonnull {
				state = "nonnull list of " + state
			} else if list {
				state = "list of " + state
			}
			return nil, fmt.Errorf("intern error (%s): %s", state, err)
		}
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

func (spec gqlSpec) internResolveFn(name string) (graphql.FieldResolveFn, error) {
	fn, ok := spec.Parser.FnMap[name]
	if !ok {
		return nil, fmt.Errorf("unknown resolve function: %s", name)
	}
	return fn, nil
}

func (ctx *gqlSpec) MakeArg(spec *gqlArg) (*graphql.ArgumentConfig, error) {
	argType, err := ctx.internType(spec.Type)
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
	in, err := ctx.internType(spec.Type)
	if err != nil {
		return nil, fmt.Errorf("field error bad type %s: %s", spec.Type, err)
	}
	fn, err := ctx.internResolveFn(spec.Resolve)
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

func (ctx *gqlSpec) MakeObject(spec *gqlObject) (*graphql.Object, error) {
	fields := graphql.Fields{}
	for key, value := range spec.FieldMap {
		field, err := ctx.MakeField(value)
		if err != nil {
			return nil, fmt.Errorf("bad field %s: %s", key, err)
		}
		fields[key] = field
	}
	object := graphql.NewObject(graphql.ObjectConfig{
		Name:   spec.Name,
		Fields: fields,
	})
	return object, nil
}

func (spec gqlSpec) MakeSchema() (*graphql.Schema, error) {
	queryType, err := spec.MakeObject(spec.query)
	if err != nil {
		return nil, fmt.Errorf("bad query: %s", err)
	}
	mutationType, err := spec.MakeObject(spec.mutation)
	if err != nil {
		return nil, fmt.Errorf("bad mudation: %s", err)
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	return &schema, err
}
