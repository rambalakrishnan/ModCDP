package modcdp

import (
	"strings"
	"testing"
)

func TestCustomCommandSchemasValidateParamsAndResults(t *testing.T) {
	cdp := New(Options{
		CustomCommands: []CustomCommand{
			{
				Name: "Custom.echo",
				ParamsSchema: map[string]any{
					"type":                 "object",
					"required":             []any{"value"},
					"properties":           map[string]any{"value": map[string]any{"type": "string"}},
					"additionalProperties": false,
				},
				ResultSchema: map[string]any{
					"type":                 "object",
					"required":             []any{"value"},
					"properties":           map[string]any{"value": map[string]any{"type": "string"}},
					"additionalProperties": false,
				},
			},
		},
	})

	if err := cdp.validateCommandParams("Custom.echo", map[string]any{"value": "ok"}); err != nil {
		t.Fatalf("expected valid params, got %v", err)
	}
	if err := cdp.validateCommandParams("Custom.echo", map[string]any{"value": 42}); err == nil || !strings.Contains(err.Error(), "paramsSchema") {
		t.Fatalf("expected params schema error, got %v", err)
	}
	if err := cdp.validateCommandResult("Custom.echo", map[string]any{"value": "ok"}); err != nil {
		t.Fatalf("expected valid result, got %v", err)
	}
	if err := cdp.validateCommandResult("Custom.echo", map[string]any{"value": 42}); err == nil || !strings.Contains(err.Error(), "resultSchema") {
		t.Fatalf("expected result schema error, got %v", err)
	}
}

func TestSchemaOnlyAddCustomCommandRegistersWithoutConnection(t *testing.T) {
	cdp := New(Options{})
	result, err := cdp.Send("Mod.addCustomCommand", map[string]any{
		"name": "Custom.clientOnly",
		"paramsSchema": map[string]any{
			"type":                 "object",
			"required":             []any{"tabId"},
			"properties":           map[string]any{"tabId": map[string]any{"type": "integer"}},
			"additionalProperties": false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	registration, ok := result.(map[string]any)
	if !ok || registration["name"] != "Custom.clientOnly" || registration["registered"] != true {
		t.Fatalf("unexpected schema-only registration result: %#v", result)
	}
	if err := cdp.validateCommandParams("Custom.clientOnly", map[string]any{"tabId": 1}); err != nil {
		t.Fatalf("expected registered schema to validate params, got %v", err)
	}
	if err := cdp.validateCommandParams("Custom.clientOnly", map[string]any{"tabId": "1"}); err == nil {
		t.Fatal("expected registered schema to reject wrong params")
	}
}

func TestCustomEventSchemasValidatePayloads(t *testing.T) {
	cdp := New(Options{
		CustomEvents: []CustomEvent{
			{
				Name: "Custom.changed",
				EventSchema: map[string]any{
					"type":                 "object",
					"required":             []any{"targetId"},
					"properties":           map[string]any{"targetId": map[string]any{"type": "string"}},
					"additionalProperties": false,
				},
			},
		},
	})

	if _, ok := cdp.validateEventData("Custom.changed", map[string]any{"targetId": "target-1"}); !ok {
		t.Fatal("expected valid event payload")
	}
	if _, ok := cdp.validateEventData("Custom.changed", map[string]any{"targetId": 1}); ok {
		t.Fatal("expected invalid event payload")
	}
}
