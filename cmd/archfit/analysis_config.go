package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/coupling-dev/archfit/internal/config"
)

// loadConfig reads doctor configuration. An absent default policy is optional.
func loadConfig(root string) (config.Config, error) {
	data, err := os.ReadFile(filepath.Join(root, defaultConfigPath)) //nolint:gosec // the operator names the policy
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.Default(), nil
		}
		return config.Config{}, err
	}
	return config.ReportConfig(data)
}
