package main

import "testing"

func TestChosenBackends(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

	bOpenAI, err := chosen(Config{Backend: "openai", Model: "gpt-5.4"})
	if err != nil {
		t.Fatalf("openai backend error: %v", err)
	}
	if bOpenAI.Name() != "openai" {
		t.Errorf("got backend name %q, want %q", bOpenAI.Name(), "openai")
	}

	bGoogle, err := chosen(Config{Backend: "google", Model: "gemini-3.7-flash"})
	if err != nil {
		t.Fatalf("google backend error: %v", err)
	}
	if bGoogle.Name() != "google" {
		t.Errorf("got backend name %q, want %q", bGoogle.Name(), "google")
	}
}

func TestChosenUnknownBackend(t *testing.T) {
	_, err := chosen(Config{Backend: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown backend, got nil")
	}
}
