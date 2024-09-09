package export

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestPathMap(t *testing.T) {

	dd := Document{
		Paths: PathSet{
			&PathItem{
				&Operation{
					OperationHeader: OperationHeader{
						Method:      "get",
						Path:        "/foo",
						OperationID: "test",
					},
				},
			},
		},
	}

	jsonVal, err := json.MarshalIndent(dd, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if val := gjson.GetBytes(jsonVal, "paths./foo.get.operationId").String(); val != "test" {
		t.Fatalf("expected operationId to be 'test', got %s", val)
	}

}
