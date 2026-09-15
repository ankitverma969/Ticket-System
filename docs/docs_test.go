package docs

import (
	"encoding/json"
	"testing"
)

func TestConfigureAPIBaseURL(t *testing.T) {
	// Test setting a custom production IP/host
	customURL := "http://3.238.130.1:8080"
	ConfigureAPIBaseURL(customURL)

	docContent := swaggerInstance.ReadDoc()
	var docMap map[string]any
	if err := json.Unmarshal([]byte(docContent), &docMap); err != nil {
		t.Fatalf("failed to unmarshal swagger doc: %v", err)
	}

	host, ok := docMap["host"].(string)
	if !ok || host != "3.238.130.1:8080" {
		t.Errorf("expected host '3.238.130.1:8080', got %v", docMap["host"])
	}

	schemes, ok := docMap["schemes"].([]any)
	if !ok || len(schemes) == 0 || schemes[0] != "http" {
		t.Errorf("expected scheme 'http', got %v", docMap["schemes"])
	}

	// Test reverting/setting to localhost default
	ConfigureAPIBaseURL("http://localhost:8080")
	docContent = swaggerInstance.ReadDoc()
	if err := json.Unmarshal([]byte(docContent), &docMap); err != nil {
		t.Fatalf("failed to unmarshal swagger doc: %v", err)
	}

	host, ok = docMap["host"].(string)
	if !ok || host != "localhost:8080" {
		t.Errorf("expected host 'localhost:8080', got %v", docMap["host"])
	}
}
