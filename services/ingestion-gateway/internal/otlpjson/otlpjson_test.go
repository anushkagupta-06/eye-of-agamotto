package otlpjson

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestHexIDsToBase64(t *testing.T) {
	in := `{"resourceSpans":[{"scopeSpans":[{"spans":[{
		"traceId":"5b8efff798038103d269b633813fc60c",
		"spanId":"eee19b7ec3c1b174",
		"parentSpanId":"",
		"startTimeUnixNano":"1700000000000000000",
		"attributes":[{"key":"spanId","value":{"stringValue":"keep-me"}}],
		"links":[{"traceId":"00112233445566778899aabbccddeeff","spanId":"0011223344556677"}],
		"droppedAttributesCount":12345678901234567890
	}]}]}]}`
	out, err := HexIDsToBase64([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	span := m["resourceSpans"].([]any)[0].(map[string]any)["scopeSpans"].([]any)[0].(map[string]any)["spans"].([]any)[0].(map[string]any)

	if got := span["traceId"].(string); got != "W47/95gDgQPSabYzgT/GDA==" {
		t.Errorf("traceId = %q", got)
	}
	raw, _ := base64.StdEncoding.DecodeString(span["spanId"].(string))
	if len(raw) != 8 {
		t.Errorf("spanId decodes to %d bytes, want 8", len(raw))
	}
	if span["parentSpanId"] != "" {
		t.Errorf("empty parent must stay empty, got %v", span["parentSpanId"])
	}
	link := span["links"].([]any)[0].(map[string]any)
	if raw, _ := base64.StdEncoding.DecodeString(link["traceId"].(string)); len(raw) != 16 {
		t.Errorf("nested link traceId decodes to %d bytes, want 16", len(raw))
	}
	if kv := span["attributes"].([]any)[0].(map[string]any); kv["key"] != "spanId" {
		t.Errorf("attribute named spanId must not be touched: %v", kv)
	}
	if !strings.Contains(string(out), "12345678901234567890") {
		t.Errorf("large number lost precision: %s", out)
	}
}

func TestErrors(t *testing.T) {
	for name, in := range map[string]string{
		"not hex":  `{"traceId":"zz"}`,
		"odd hex":  `{"spanId":"abc"}`,
		"trailing": `{} {}`,
		"garbage":  `not json`,
		"empty":    ``,
	} {
		if _, err := HexIDsToBase64([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}