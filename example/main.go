package main

import (
	gqlhandler "github.com/graphql-go/graphql-go-handler"
	"net/http"
)

func main() {

	schema, err := initSchema()
	if err != nil {
		panic(err)
	}

	// create a graphl-go HTTP handler with our previously defined schema
	// and we also set it to return pretty JSON output
	h := gqlhandler.New(&gqlhandler.Config{
		Schema: schema,
		Pretty: true,
	})

	// serve a GraphQL endpoint at `/graphql`
	http.Handle("/graphql", h)

	// and serve!
	http.ListenAndServe(":8080", nil)
}
