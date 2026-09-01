package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/spec"
)

type manifestChange struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

func printJobPlan(w io.Writer, current *api.JobStatusResponse, exists bool, desired *spec.JobSpec) error {
	if !exists || current == nil || current.Spec == nil {
		plan := struct {
			Action             string `json:"action"`
			Namespace          string `json:"namespace"`
			Job                string `json:"job"`
			DesiredAllocations int    `json:"desired_allocations"`
		}{"create", desired.Namespace, desired.Name, desiredAllocations(desired)}
		if config.Output == "json" {
			return writeJSON(w, plan)
		}
		_, err := fmt.Fprintf(w, "Plan: create %s/%s (%d desired allocations)\n", desired.Namespace, desired.Name, plan.DesiredAllocations)
		return err
	}

	changes := diffJobSpecs(current.Spec, desired)
	if config.Output == "json" {
		action := "update"
		if len(changes) == 0 {
			action = "none"
		}
		return writeJSON(w, struct {
			Action          string           `json:"action"`
			Namespace       string           `json:"namespace"`
			Job             string           `json:"job"`
			CurrentRevision int              `json:"current_revision"`
			Changes         []manifestChange `json:"changes"`
		}{action, desired.Namespace, desired.Name, current.Revision, changes})
	}
	if len(changes) == 0 {
		_, err := fmt.Fprintf(w, "Plan: no changes to %s/%s (revision %d)\n", desired.Namespace, desired.Name, current.Revision)
		return err
	}
	if _, err := fmt.Fprintf(w, "Plan: update %s/%s from revision %d\n", desired.Namespace, desired.Name, current.Revision); err != nil {
		return err
	}
	for _, change := range changes {
		switch change.Operation {
		case "add":
			if _, err := fmt.Fprintf(w, "  + %s: %s\n", change.Path, formatChangeValue(change.Path, change.After)); err != nil {
				return err
			}
		case "remove":
			if _, err := fmt.Fprintf(w, "  - %s: %s\n", change.Path, formatChangeValue(change.Path, change.Before)); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(w, "  ~ %s: %s -> %s\n", change.Path, formatChangeValue(change.Path, change.Before), formatChangeValue(change.Path, change.After)); err != nil {
				return err
			}
		}
	}
	return nil
}

func diffJobSpecs(before, after *spec.JobSpec) []manifestChange {
	var left any
	var right any
	leftRaw, _ := json.Marshal(before)
	rightRaw, _ := json.Marshal(after)
	_ = json.Unmarshal(leftRaw, &left)
	_ = json.Unmarshal(rightRaw, &right)
	changes := make([]manifestChange, 0)
	walkManifestDiff("", left, right, &changes)
	return changes
}

func walkManifestDiff(path string, before, after any, changes *[]manifestChange) {
	if reflect.DeepEqual(before, after) {
		return
	}
	leftMap, leftOK := before.(map[string]any)
	rightMap, rightOK := after.(map[string]any)
	if leftOK && rightOK {
		keys := make(map[string]struct{}, len(leftMap)+len(rightMap))
		for key := range leftMap {
			keys[key] = struct{}{}
		}
		for key := range rightMap {
			keys[key] = struct{}{}
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			left, leftExists := leftMap[key]
			right, rightExists := rightMap[key]
			child := joinManifestPath(path, key)
			switch {
			case !leftExists:
				*changes = append(*changes, manifestChange{Operation: "add", Path: child, After: right})
			case !rightExists:
				*changes = append(*changes, manifestChange{Operation: "remove", Path: child, Before: left})
			default:
				walkManifestDiff(child, left, right, changes)
			}
		}
		return
	}

	leftSlice, leftOK := before.([]any)
	rightSlice, rightOK := after.([]any)
	if leftOK && rightOK {
		if leftNamed, okLeft := namedManifestSlice(leftSlice); okLeft {
			if rightNamed, okRight := namedManifestSlice(rightSlice); okRight {
				names := make(map[string]struct{}, len(leftNamed)+len(rightNamed))
				for name := range leftNamed {
					names[name] = struct{}{}
				}
				for name := range rightNamed {
					names[name] = struct{}{}
				}
				ordered := make([]string, 0, len(names))
				for name := range names {
					ordered = append(ordered, name)
				}
				sort.Strings(ordered)
				for _, name := range ordered {
					left, leftExists := leftNamed[name]
					right, rightExists := rightNamed[name]
					child := fmt.Sprintf("%s[%s]", path, name)
					switch {
					case !leftExists:
						*changes = append(*changes, manifestChange{Operation: "add", Path: child, After: right})
					case !rightExists:
						*changes = append(*changes, manifestChange{Operation: "remove", Path: child, Before: left})
					default:
						walkManifestDiff(child, left, right, changes)
					}
				}
				return
			}
		}
		limit := len(leftSlice)
		if len(rightSlice) > limit {
			limit = len(rightSlice)
		}
		for i := 0; i < limit; i++ {
			child := fmt.Sprintf("%s[%d]", path, i)
			switch {
			case i >= len(leftSlice):
				*changes = append(*changes, manifestChange{Operation: "add", Path: child, After: rightSlice[i]})
			case i >= len(rightSlice):
				*changes = append(*changes, manifestChange{Operation: "remove", Path: child, Before: leftSlice[i]})
			default:
				walkManifestDiff(child, leftSlice[i], rightSlice[i], changes)
			}
		}
		return
	}
	*changes = append(*changes, manifestChange{Operation: "change", Path: path, Before: before, After: after})
}

func namedManifestSlice(values []any) (map[string]any, bool) {
	result := make(map[string]any, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		name, ok := item["name"].(string)
		if !ok || name == "" {
			return nil, false
		}
		if _, duplicate := result[name]; duplicate {
			return nil, false
		}
		result[name] = value
	}
	return result, true
}

func joinManifestPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func formatChangeValue(path string, value any) string {
	if value == nil {
		return "null"
	}
	if number, ok := value.(float64); ok && isDurationPath(path) {
		return time.Duration(int64(number)).String()
	}
	switch typed := value.(type) {
	case string:
		return fmt.Sprintf("%q", typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(raw)
	}
}

func isDurationPath(path string) bool {
	return strings.HasSuffix(path, ".window") || strings.HasSuffix(path, ".interval") || strings.HasSuffix(path, ".timeout")
}
