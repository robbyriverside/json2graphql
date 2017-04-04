package json2graphql

import (
	"testing"
)

func TestRecursion(t *testing.T) {
	maker := NewMaker()

	maker.Query("top", `{
        "type": "[middle!]"
    }`)

	maker.Field("middle", "one", `{
        "type": "[bottom]"
    }`)

	maker.Field("bottom", "one", `{
        "type": "[deep!]!"
    }`)

	maker.Field("deep", "one", `{
        "type": "[middle]!"
    }`)

	_, err := maker.MakeSchema()
	if err == nil {
		t.Fatalf("recursion not found")
	}
	t.Logf("recursion: %s", err)
}
