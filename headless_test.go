package main

import (
	"os"
	"strings"
	"testing"
)

func TestStripPrintFlagExtractsPrompt(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		want     string
		wantArgs string
	}{
		{"-print with arg", []string{"nacelle", "-print", "hello world"}, "hello world", "nacelle"},
		{"-print=equals", []string{"nacelle", "-print=hello"}, "hello", "nacelle"},
		{"-print alone (stdin)", []string{"nacelle", "-print"}, "", "nacelle"},
		{"no -print", []string{"nacelle", "-model", "abc"}, "", "nacelle -model abc"},
		{"-print after other flags", []string{"nacelle", "-root", ".", "-print", "hello"}, "hello", "nacelle -root ."},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Args
			os.Args = tt.args
			got := stripPrintFlag()
			gotArgs := strings.Join(os.Args, " ")
			os.Args = saved

			if got != tt.want {
				t.Errorf("stripPrintFlag() = %q, want %q", got, tt.want)
			}
			if gotArgs != tt.wantArgs {
				t.Errorf("after strip, os.Args = %q, want %q", gotArgs, tt.wantArgs)
			}
		})
	}
}
