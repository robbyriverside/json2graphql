package main

import (
	"github.com/graphql-go/graphql"
)

var values = make(map[string]string)

func getValue(p graphql.ResolveParams) (interface{}, error) {
	name, ok := p.Args["name"].(string)
	if !ok {
		name = ""
	}
	return values[name], nil
}

func putValue(p graphql.ResolveParams) (interface{}, error) {
	name, ok := p.Args["name"].(string)
	if !ok {
		name = ""
	}
	value, ok := p.Args["value"].(string)
	if !ok {
		value = ""
	}
	values[name] = value
	return values[name], nil
}
