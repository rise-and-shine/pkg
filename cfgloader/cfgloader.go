// Package cfgloader provides a simple way to load and validate configuration at the start of an application.
package cfgloader

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const (
	EnvProduction = "production"
	EnvStaging    = "staging"
	EnvDev        = "dev"
	EnvLocal      = "local"
	EnvTest       = "test"
)

// MustLoad loads and validates configuration from a YAML file based on the ENVIRONMENT variable.
// The files must be named in the format ${ENVIRONMENT}.yaml and located in the config directory at the root of the project.
//
// The configuration struct should use `yaml` struct tags to map fields to the YAML file structure.
//
// Default values for configuration fields can be set using the `default` struct tag. These values are applied before validation
// if the corresponding fields are not explicitly defined in the YAML file.
//
// Validations are done using the go-playground/validator package.
// See https://pkg.go.dev/github.com/go-playground/validator/v10 for more information.
//
// Example:
//
//	type Config struct {
//	    Host        string `yaml:"host" validate:"required"`  // Maps to the "host" field in the YAML file, required
//	    Port        int    `yaml:"port" default:"8080"`       // Maps to the "port" field in the YAML file, defaults to 8080
//	    LogLevel    string `yaml:"log_level" default:"info"`  // Maps to the "log_level" field, defaults to "info"
//	}
//
// If the YAML file does not define these fields, the default values will be applied.
func MustLoad[T any]() T {
	var config T

	ensureNotPointer(config)

	_ = godotenv.Load()

	env := defineEnvironment()

	configPath := buildConfigPath(env)

	data := readConfigFile(configPath)

	data = replaceEnvVars(data)

	unmarshalConfig(data, &config, env)

	setDefaults(&config)

	validateConfig(&config, env)

	return config
}

func ensureNotPointer(config any) {
	if reflect.ValueOf(config).Kind() == reflect.Ptr {
		slog.Error("[cfgloader]: arg config must not be a pointer")
		os.Exit(1)
	}
}

func defineEnvironment() string {
	env := os.Getenv("ENVIRONMENT")
	if !slices.Contains([]string{EnvProduction, EnvStaging, EnvDev, EnvLocal, EnvTest}, env) {
		slog.Error(
			"[cfgloader]: ENVIRONMENT env variable is not set or invalid. Choices are: production, staging, dev, local, test",
		)
		os.Exit(1)
	}
	return env
}

func buildConfigPath(env string) string {
	return fmt.Sprintf("./config/%s.yaml", env)
}

func readConfigFile(path string) []byte {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		slog.Error(
			fmt.Sprintf(
				"[cfgloader]: config file not found in the path %s - Make sure that the yaml file exists for each environment",
				path,
			),
		)
		os.Exit(1)
	}
	if err != nil {
		slog.Error(
			fmt.Sprintf("[cfgloader]: failed to read config file %s: %v", path, err),
		)
		os.Exit(1)
	}

	return data
}

func replaceEnvVars(data []byte) []byte {
	dataStr := os.ExpandEnv(string(data))
	return []byte(dataStr)
}

func unmarshalConfig(data []byte, config any, env string) {
	err := yaml.Unmarshal(data, config)
	if err != nil {
		slog.Error(
			fmt.Sprintf("[cfgloader]: failed to unmarshal %s config file: %v", env, err),
		)
		os.Exit(1)
	}
}

func setDefaults(config any) {
	if err := defaults.Set(config); err != nil {
		slog.Error(
			fmt.Sprintf("[cfgloader]: failed to set default values for config: %s", err),
		)
		os.Exit(1)
	}
}

func validateConfig(config any, env string) {
	v := validator.New(validator.WithRequiredStructEnabled())
	err := v.Struct(config)

	failedFields := make([]string, 0)
	if errs, ok := err.(validator.ValidationErrors); ok { //nolint: errorlint // Using type assertion for validator errors handling
		for _, err := range errs {
			tagErr := err.Tag()
			if err.Param() != "" {
				tagErr += fmt.Sprintf("=%s", err.Param())
			}
			failedFields = append(failedFields, fmt.Sprintf("%s: %s", err.Namespace(), tagErr))
		}
	}

	if len(failedFields) > 0 {
		slog.Error(
			fmt.Sprintf("[cfgloader]: invalid fields in %s config -> %s", env, strings.Join(failedFields, ",  ")),
		)
		os.Exit(1)
	}
}
