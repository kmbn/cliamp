package config

import (
	"testing"
)

func TestParseStringEnvInterpolation(t *testing.T) {
	t.Setenv("CLIAMP_TEST_VAR", "from-env")
	t.Setenv("CLIAMP_TEST_EMPTY", "")

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain quoted string", `"hello"`, "hello"},
		{"plain single-quoted", `'hello'`, "hello"},
		{"unquoted plain", `hello`, "hello"},
		{"dollar braces set", `"${CLIAMP_TEST_VAR}"`, "from-env"},
		{"dollar bare set", `"$CLIAMP_TEST_VAR"`, "from-env"},
		{"unquoted dollar braces", `${CLIAMP_TEST_VAR}`, "from-env"},
		{"unset var returns empty", `"${CLIAMP_NOT_SET_XYZ}"`, ""},
		{"empty var returns empty", `"${CLIAMP_TEST_EMPTY}"`, ""},
		{"literal dollar in middle preserved", `"p@$$w0rd"`, "p@$$w0rd"},
		{"literal dollar at start with non-name", `"$1abc"`, "$1abc"},
		{"unmatched brace left alone", `"${UNCLOSED"`, "${UNCLOSED"},
		{"only dollar", `"$"`, "$"},
		{"interpolation only on whole value", `"prefix-$CLIAMP_TEST_VAR"`, "prefix-$CLIAMP_TEST_VAR"},
		{"underscore-leading name", `"$_CLIAMP_TEST"`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseString(tt.in)
			if got != tt.want {
				t.Fatalf("parseString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
