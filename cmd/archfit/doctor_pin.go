package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const unknownWorkflowPin = "cannot tell whether the local engine matches the workflow pin"

var pinComment = regexp.MustCompile(`^[ \t]*#[ \t]+(v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)[ \t]*$`)

func doctorPin(root, local string) string {
	entries, err := os.ReadDir(filepath.Join(root, workflowsDir))
	if err != nil {
		return unknownWorkflowPin
	}
	var pin string
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); !e.Type().IsRegular() || (ext != ".yml" && ext != ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, workflowsDir, e.Name())) //nolint:gosec // local workflow
		if err != nil {
			return unknownWorkflowPin
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			line = strings.TrimSuffix(line, "\r")
			at := usesAction.FindStringIndex(line)
			if at == nil {
				continue
			}
			ref, suffix, ok := strings.Cut(line[at[1]:], "#")
			if !ok || len(strings.Fields(ref)) != 1 || strings.TrimRight(ref, " \t") == ref {
				return unknownWorkflowPin
			}
			match := pinComment.FindStringSubmatch("#" + suffix)
			if match == nil || (pin != "" && pin != match[1]) {
				return unknownWorkflowPin
			}
			pin = match[1]
		}
	}
	if pin == "" || local == devVersion {
		return unknownWorkflowPin
	}
	if strings.TrimPrefix(pin, "v") == strings.TrimPrefix(local, "v") {
		return "the local engine matches the workflow pin"
	}
	return "the local engine does not match the workflow pin (" + local + " vs " + pin + ")"
}
