package docs

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var swaggerJSON string

var (
	swaggerInstance = &SwaggerDoc{
		rawJSON: swaggerJSON,
	}
	once sync.Once
)

type SwaggerDoc struct {
	mu      sync.RWMutex
	rawJSON string
	docJSON string
}

func (s *SwaggerDoc) ReadDoc() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.docJSON != "" {
		return s.docJSON
	}
	return s.rawJSON
}

// ConfigureAPIBaseURL dynamically updates the Swagger host, schemes, and basePath
// according to the provided API_BASE_URL (e.g., "http://3.238.130.1:8080" or "http://localhost:8080").
func ConfigureAPIBaseURL(apiBaseURL string) {
	swaggerInstance.mu.Lock()
	defer swaggerInstance.mu.Unlock()

	trimmed := strings.TrimSpace(apiBaseURL)
	if trimmed == "" {
		trimmed = "http://localhost:8080"
	}

	// If missing scheme, prepend http:// for proper URL parsing
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return
	}

	var docMap map[string]any
	if err := json.Unmarshal([]byte(swaggerInstance.rawJSON), &docMap); err != nil {
		return
	}

	if parsed.Host != "" {
		docMap["host"] = parsed.Host
	}
	if parsed.Scheme != "" {
		docMap["schemes"] = []string{parsed.Scheme}
	}
	if parsed.Path != "" && parsed.Path != "/" {
		docMap["basePath"] = parsed.Path
	} else {
		docMap["basePath"] = "/"
	}

	updatedBytes, err := json.MarshalIndent(docMap, "", "  ")
	if err == nil {
		swaggerInstance.docJSON = string(updatedBytes)
	}
}

func init() {
	once.Do(func() {
		swag.Register(swag.Name, swaggerInstance)
	})
}
