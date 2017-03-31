# json2graphql
Simplified Golang GraphQL development using a json syntax to describe objects and fields

A *work-in-progress* implementation of json syntax for defining GraphQL queries.

### Documentation

*TBD*

### Getting Started

To install the library, run:
```bash
go get github.com/robbyriverside/json2graphql
```

The following example creates a simple get-put API to a web server using GraphQL.

```go
package main

import (
    "fmt"
    "github.com/graphql-go/graphql"
    j2g "github.com/robbyriverside/json2graphql"
)

var maker = j2g.NewMaker()

func initSchema() (*graphql.Schema, error) {

    //This is a single top-level GraphQL mutation.
    //These can be distributed around your code.
    //They are combined in the maker object
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
            "getValue": func(p graphql.ResolveParams) (interface{}, error) {
                name, ok := p.Args["name"].(string)
                if !ok {
                    name = ""
                }
                return values[name], nil
            },
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
            "putValue": func(p graphql.ResolveParams) (interface{}, error) {
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
            },
        },
    )

    //MakeSchema combines all the top-level objects and produces a GraphQL schema
    //Even after the schema is made, more definitions could be added and the schema made again.
    schema, err := maker.MakeSchema()
    if err != nil {
        return nil, fmt.Errorf("unable to make schema: %s", err)
    }
    return schema, nil
}

func main() {
    schema, err := initSchema()
    if err != nil {
        panic(err)
    }

    h := gqlhandler.New(&gqlhandler.Config{
        Schema: schema,
        Pretty: true,
    })

    http.Handle("/graphql", h)
    http.ListenAndServe(":8080", nil)
}

```

### Json Syntax

The Json syntax reflects the structure of the code used to write a query using the GraphQL Go package.
But hides the implementation details and makes for an easier to read specification.

For example:

```go
//This is verbose and repetative.
var queryType = graphql.NewObject(graphql.ObjectConfig{
    Name: "Query",
    Fields: graphql.Fields{
        "get": &graphql.Field{
            Args: graphql.FieldConfigArgument{
                "name": &graphql.ArgumentConfig{
                    Type: graphql.String,
                },
            },
            Type:        graphql.String,
            Resolve:     getValue,
        },
    },
    //Other field definitions would go here
})

//The same definition using json2graphql
//The final Query is composed of many of these
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

```

### Third Party Libraries
| Name          | Author        | Description  |
|:-------------:|:-------------:|:------------:|
| [graphql-go-handler](https://github.com/graphql-go/graphql-go-handler) | [Hafiz Ismail](https://github.com/sogko) | Middleware to handle GraphQL queries through HTTP requests. |

### Roadmap

- [ ] Replace Json syntax with GraphQL Schema syntax.
