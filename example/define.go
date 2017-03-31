package main

import (
	"fmt"
	"github.com/graphql-go/graphql"
	j2g "github.com/robbyriverside/json2graphql"
)

func initSchema() (*graphql.Schema, error) {
	maker := j2g.NewMaker()

	maker.Query("get",
		`{
            "args": {
                "name": {
                    "type": "String"
                }
            },
            "type": "String",
            "resolve": "getValue"
        }`,
		j2g.ResolveMap{
			"getValue": getValue,
		},
	)

	maker.Mutation("put",
		`{
            "args": {
                "name": {
                    "type": "String!"
                },
                "value": {
                    "type": "String"
                }
            },
            "type": "String",
            "resolve": "putValue"
        }`,
		j2g.ResolveMap{
			"putValue": putValue,
		},
	)

	schema, err := maker.MakeSchema()
	if err != nil {
		return nil, fmt.Errorf("unable to make schema: %s", err)
	}
	return schema, nil
}
