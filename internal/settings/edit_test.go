package settings_test

import (
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/settings"
)

func TestApplyReplacesAValueInPlaceAndKeepsEverythingElseByteForByte(t *testing.T) {
	doc := "{\n  \"theme\": \"dark\",\n  \"model\": \"opus\",\n  \"env\": { \"A\": \"1\" },\n  \"permissions\": {\n    \"allow\": [\"x\"]\n  }\n}\n"

	got, err := settings.Apply([]byte(doc), []settings.Edit{{Key: "model", Raw: `"sonnet"`}})

	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(doc, `"opus"`, `"sonnet"`, 1)
	if string(got) != want {
		t.Fatalf("Apply()\n got: %q\nwant: %q", got, want)
	}
}

func TestApplyReplacesAnObjectKeepingForeignFormatting(t *testing.T) {
	doc := "{\n  \"env\": { \"A\": \"1\" },\n  \"permissions\": {\n    \"allow\": [\"x\"]\n  },\n  \"theme\": \"dark\"\n}\n"

	got, err := settings.Apply([]byte(doc), []settings.Edit{{Key: "permissions", Raw: "{\n    \"deny\": []\n  }"}})

	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"env\": { \"A\": \"1\" },\n  \"permissions\": {\n    \"deny\": []\n  },\n  \"theme\": \"dark\"\n}\n"
	if string(got) != want {
		t.Fatalf("Apply()\n got: %q\nwant: %q", got, want)
	}
}

func TestApplyAddsAKeyInTheDocumentsOwnStyle(t *testing.T) {
	cases := map[string]struct{ doc, want string }{
		"two spaces":  {"{\n  \"a\": 1\n}\n", "{\n  \"a\": 1,\n  \"model\": \"sonnet\"\n}\n"},
		"four spaces": {"{\n    \"a\": 1,\n    \"b\": [1, 2]\n}", "{\n    \"a\": 1,\n    \"b\": [1, 2],\n    \"model\": \"sonnet\"\n}"},
		"tabs":        {"{\n\t\"a\": 1\n}\n", "{\n\t\"a\": 1,\n\t\"model\": \"sonnet\"\n}\n"},
		"compact":     {`{"a":1}`, `{"a":1,"model":"sonnet"}`},
		"crlf":        {"{\r\n  \"a\": 1\r\n}\r\n", "{\r\n  \"a\": 1,\r\n  \"model\": \"sonnet\"\r\n}\r\n"},
		"nested last": {"{\n  \"a\": {\n    \"x\": 1\n  }\n}\n", "{\n  \"a\": {\n    \"x\": 1\n  },\n  \"model\": \"sonnet\"\n}\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := settings.Apply([]byte(c.doc), []settings.Edit{{Key: "model", Raw: `"sonnet"`}})

			if err != nil {
				t.Fatal(err)
			}
			if string(got) != c.want {
				t.Fatalf("Apply()\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

func TestApplyAddThenDeleteRestoresTheOriginalBytes(t *testing.T) {
	docs := []string{
		"{\n  \"a\": 1\n}\n",
		"{\n    \"a\": 1,\n    \"b\": [1, 2]\n}",
		"{\n\t\"a\": 1\n}\n",
		`{"a":1}`,
		"{\r\n  \"a\": 1\r\n}\r\n",
		"{\n  \"env\": { \"A\": \"1\" },\n  \"permissions\": {\n    \"allow\": [\"x\"]\n  }\n}\n",
	}
	for _, doc := range docs {
		added, err := settings.Apply([]byte(doc), []settings.Edit{{Key: "model", Raw: `"sonnet"`}})
		if err != nil {
			t.Fatal(err)
		}

		back, err := settings.Apply(added, []settings.Edit{{Key: "model", Delete: true}})

		if err != nil || string(back) != doc {
			t.Errorf("round trip of %q\n got: %q (err %v)", doc, back, err)
		}
	}
}

func TestApplyCreatesADocumentWhenThereIsNone(t *testing.T) {
	got, err := settings.Apply(nil, []settings.Edit{
		{Key: "model", Raw: `"sonnet"`},
		{Key: "effortLevel", Raw: `"high"`},
	})

	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"model\": \"sonnet\",\n  \"effortLevel\": \"high\"\n}\n"
	if string(got) != want {
		t.Fatalf("Apply()\n got: %q\nwant: %q", got, want)
	}
}

func TestApplyDeletingAMissingKeyChangesNothing(t *testing.T) {
	doc := "{\n  \"a\": 1\n}\n"

	got, err := settings.Apply([]byte(doc), []settings.Edit{{Key: "model", Delete: true}})

	if err != nil || string(got) != doc {
		t.Fatalf("Apply() = %q, %v", got, err)
	}
}

func TestApplyKeepsTheOrderOfForeignKeys(t *testing.T) {
	doc := "{\n  \"z\": 1,\n  \"model\": \"opus\",\n  \"a\": 2\n}\n"

	got, err := settings.Apply([]byte(doc), []settings.Edit{
		{Key: "model", Delete: true},
		{Key: "agent", Raw: `"reviewer"`},
	})

	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"z\": 1,\n  \"a\": 2,\n  \"agent\": \"reviewer\"\n}\n"
	if string(got) != want {
		t.Fatalf("Apply()\n got: %q\nwant: %q", got, want)
	}
}

func TestApplyRejectsInvalidAndNonObjectDocuments(t *testing.T) {
	for _, doc := range []string{`{"a": `, `[1, 2]`, `"text"`} {
		if _, err := settings.Apply([]byte(doc), []settings.Edit{{Key: "model", Raw: `"x"`}}); err == nil {
			t.Errorf("Apply(%q) accepted an unusable document", doc)
		}
	}
}

func TestApplyRejectsRawValuesThatAreNotJSON(t *testing.T) {
	_, err := settings.Apply([]byte("{}\n"), []settings.Edit{{Key: "model", Raw: `sonnet`}})

	if err == nil {
		t.Fatal("Apply() accepted a raw value that is not valid JSON")
	}
}

func TestEqualJSONComparesValuesNotFormatting(t *testing.T) {
	if !settings.EqualJSON(`{"a": [1, 2], "b": "x"}`, "{\n \"b\": \"x\",\n \"a\":[1,2]\n}") {
		t.Error("EqualJSON() = false for the same value in different formatting")
	}
	if settings.EqualJSON(`{"a": 1}`, `{"a": 2}`) {
		t.Error("EqualJSON() = true for different values")
	}
	if settings.EqualJSON(`"opus"`, `"opus[1m]"`) {
		t.Error("EqualJSON() = true for different strings")
	}
}
