package bootstrap

import (
	"os"
	"strings"
)

type envVar struct {
	Name string
	Val  string
}

func setEnvVars(vars []string) error {
	envVars := parseEnvVars(vars)
	for _, v := range envVars {
		if err := os.Setenv(v.Name, v.Val); err != nil {
			return err
		}
	}

	return nil
}

func parseEnvVars(vars []string) (envVars []envVar) {
	for _, v := range vars {
		pair := strings.Split(v, "=")
		if len(pair) != 2 {
			continue
		}
		envVars = append(envVars, envVar{Name: pair[0], Val: pair[1]})
	}

	return
}
