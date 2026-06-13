package okf

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPrintSchema(t *testing.T) {
	s := SkillSchema{
		Name:        "okf-demo",
		Description: "demo",
		Commands: []CommandSchema{
			{Name: "produce", Description: "make", Flags: []FlagSchema{
				{Name: "out", Type: "string", Description: "output dir", Required: true},
			}},
		},
	}
	var buf bytes.Buffer
	if err := PrintSchema(&buf, s); err != nil {
		t.Fatalf("PrintSchema error: %v", err)
	}
	var round SkillSchema
	if err := json.Unmarshal(buf.Bytes(), &round); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if round.Name != "okf-demo" || len(round.Commands) != 1 {
		t.Fatalf("round-trip mismatch: %+v", round)
	}
	if round.Commands[0].Flags[0].Name != "out" || !round.Commands[0].Flags[0].Required {
		t.Fatalf("flag round-trip mismatch: %+v", round.Commands[0].Flags)
	}
}
