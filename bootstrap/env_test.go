package bootstrap

import (
	"reflect"
	"testing"
)

func TestEnvVarParser(t *testing.T) {
	testsSet := []struct {
		Description string
		EnvVars     []string
		Expected    []envVar
	}{
		{
			Description: "Test Empty list",
			EnvVars:     nil,
		},
		{
			Description: "All invalid format",
			EnvVars:     []string{"invalid_format"},
		},
		{
			Description: "One with invalid format",
			EnvVars:     []string{"invalid_format", "VAR1=1"},
			Expected:    []envVar{{Name: "VAR1", Val: "1"}},
		},
		{
			Description: "All valid",
			EnvVars:     []string{"VAR1=1", "VAR2=2"},
			Expected:    []envVar{{Name: "VAR1", Val: "1"}, {Name: "VAR2", Val: "2"}},
		},
	}

	for _, test := range testsSet {
		r := parseEnvVars(test.EnvVars)
		if !reflect.DeepEqual(r, test.Expected) {
			t.Errorf("%s: expected %q, got %q", test.Description, test.Expected, r)
		}
	}
}
