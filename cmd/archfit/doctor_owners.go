package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/coupling-dev/archfit/internal/config/policyfile"
	"github.com/coupling-dev/archfit/internal/history/identity"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

func doctorOwners(ctx context.Context, root string, runner toolrun.Runner) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, defaultConfigPath)) //nolint:gosec // policy belongs to the checkout
	if err != nil {
		return "", nil
	}
	file, err := policyfile.Decode(data)
	if err != nil {
		return "", nil
	}
	person, unknown := identity.PersonOwner(ctx, root, runner, file.Owners)
	if person {
		return "", nil
	}
	if unknown {
		return "owner not verified\nnext: verify that owners includes a person's GitHub login", nil
	}
	return "no person is named as an owner\nnext: put a person's GitHub login in owners in .archfit.yaml", errors.New("policy has no person as an owner")
}
