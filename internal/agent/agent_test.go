package agent

import "testing"

func TestChosenBackends(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

	bOpenAI, err := chosen(Config{Provider: Provider{Backend: "openai", Model: "gpt-5.4"}})
	if err != nil {
		t.Fatalf("openai backend error: %v", err)
	}
	if bOpenAI.Name() != "openai" {
		t.Errorf("got backend name %q, want %q", bOpenAI.Name(), "openai")
	}

	bGoogle, err := chosen(Config{Provider: Provider{Backend: "google", Model: "gemini-3.7-flash"}})
	if err != nil {
		t.Fatalf("google backend error: %v", err)
	}
	if bGoogle.Name() != "google" {
		t.Errorf("got backend name %q, want %q", bGoogle.Name(), "google")
	}
}

// A custom provider forwards its base URL and its own API key to the chosen
// backend: the key the config carries is what reaches the gateway, not the
// OPENAI_API_KEY variable. Emptying the variable here proves the key travelled
// in the Config rather than being read from the environment, which is the
// whole difference a freellmapi-style gateway is used for.
func TestChosenForwardsCustomBaseURLAndKeyToOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	b, err := chosen(Config{Provider: Provider{
		Backend: "openai",
		BaseURL: "http://localhost:3001/v1",
		APIKey:  "sk-gateway",
		Model:   "auto",
	}})
	if err != nil {
		t.Fatalf("openai backend error: %v", err)
	}
	if b.Name() != "openai" {
		t.Errorf("got backend name %q, want openai", b.Name())
	}
}

func TestChosenUnknownBackend(t *testing.T) {
	_, err := chosen(Config{Provider: Provider{Backend: "unknown"}})
	if err == nil {
		t.Fatal("expected error for unknown backend, got nil")
	}
}

// anthropic cannot point at a custom endpoint: it takes a pre-built client,
// not a URL and key. Setting base_url or api_key against it must be a loud
// error, not a silently-ignored setting.
func TestChosenRejectsCustomEndpointOnAnthropic(t *testing.T) {
	_, err := chosen(Config{Provider: Provider{Backend: "anthropic", BaseURL: "http://localhost:3001/v1"}})
	if err == nil {
		t.Fatal("expected error for base_url on anthropic, got nil")
	}
	_, err = chosen(Config{Provider: Provider{Backend: "anthropic", APIKey: "sk-x"}})
	if err == nil {
		t.Fatal("expected error for api_key on anthropic, got nil")
	}
}
