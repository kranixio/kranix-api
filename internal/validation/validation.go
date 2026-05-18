package validation

import (
	"regexp"
	"strings"
	"time"

	"github.com/kranix-io/kranix-packages/errors"
	"github.com/kranix-io/kranix-packages/types"
	"github.com/robfig/cron/v3"
)

// ValidateWorkloadSpec validates a workload specification.
func ValidateWorkloadSpec(spec *types.WorkloadSpec) error {
	if spec.Image == "" {
		return errors.Wrap(errors.ErrInvalidSpec, "workload image is required")
	}

	if spec.Replicas < 0 {
		return errors.Wrap(errors.ErrInvalidSpec, "workload replicas must be non-negative")
	}

	if spec.Backend == "" {
		return errors.Wrap(errors.ErrInvalidSpec, "workload backend is required")
	}

	// Validate backend type
	if spec.Backend != "docker" && spec.Backend != "kubernetes" {
		return errors.Wrap(errors.ErrInvalidSpec, "backend must be 'docker' or 'kubernetes'")
	}

	// Validate image format
	if !isValidImageName(spec.Image) {
		return errors.Wrap(errors.ErrInvalidSpec, "invalid image format")
	}

	if spec.Scheduling != nil && spec.Scheduling.WorkloadPriority != "" {
		switch strings.ToLower(strings.TrimSpace(spec.Scheduling.WorkloadPriority)) {
		case "critical", "high", "normal", "low":
		default:
			return errors.Wrap(errors.ErrInvalidSpec, "scheduling.workloadPriority must be critical|high|normal|low")
		}
	}

	if spec.CronSchedule != nil && !spec.CronSchedule.Suspended {
		schedule := strings.TrimSpace(spec.CronSchedule.Schedule)
		if schedule == "" {
			return errors.Wrap(errors.ErrInvalidSpec, "cronSchedule.schedule is required when cron is enabled")
		}
		if _, err := cron.ParseStandard(schedule); err != nil {
			return errors.Wrap(errors.ErrInvalidSpec, "invalid cronSchedule.schedule (use standard 5-field cron)")
		}
		if tz := strings.TrimSpace(spec.CronSchedule.TimeZone); tz != "" {
			if _, err := time.LoadLocation(tz); err != nil {
				return errors.Wrap(errors.ErrInvalidSpec, "invalid cronSchedule.timeZone")
			}
		}
		if cp := strings.ToLower(strings.TrimSpace(spec.CronSchedule.ConcurrencyPolicy)); cp != "" {
			switch cp {
			case "allow", "forbid", "replace":
			default:
				return errors.Wrap(errors.ErrInvalidSpec, "cronSchedule.concurrencyPolicy must be allow|forbid|replace")
			}
		}
	}

	return nil
}

// ValidateNamespace validates a namespace.
func ValidateNamespace(namespace *types.Namespace) error {
	if namespace.Name == "" {
		return errors.Wrap(errors.ErrInvalidSpec, "namespace name is required")
	}

	if !isValidResourceName(namespace.Name) {
		return errors.Wrap(errors.ErrInvalidSpec, "invalid namespace name")
	}

	return nil
}

// isValidImageName checks if an image name is valid.
func isValidImageName(image string) bool {
	// Basic validation for image names (e.g., nginx:latest, registry.example.com/myimage:1.0)
	imagePattern := regexp.MustCompile(`^[a-z0-9]+([\.\-_][a-z0-9]+)*(\/[a-z0-9]+([\.\-_][a-z0-9]+)*)*(:[a-zA-Z0-9\.\-_]+)?$`)
	return imagePattern.MatchString(image)
}

// isValidResourceName checks if a resource name is valid.
func isValidResourceName(name string) bool {
	// Kubernetes-style resource name validation
	namePattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if len(name) > 253 {
		return false
	}
	return namePattern.MatchString(name)
}

// ValidateLogOptions validates log streaming options.
func ValidateLogOptions(opts *types.LogOptions) error {
	if opts.Tail < 0 {
		return errors.Wrap(errors.ErrInvalidSpec, "tail must be non-negative")
	}

	if opts.Tail > 10000 {
		return errors.Wrap(errors.ErrInvalidSpec, "tail cannot exceed 10000")
	}

	return nil
}

// SanitizeInput sanitizes user input to prevent injection attacks.
func SanitizeInput(input string) string {
	// Remove potentially dangerous characters
	input = strings.TrimSpace(input)
	input = strings.ReplaceAll(input, "\n", "")
	input = strings.ReplaceAll(input, "\r", "")
	input = strings.ReplaceAll(input, "\t", "")
	return input
}

// ValidateID validates a resource ID.
func ValidateID(id string) error {
	if id == "" {
		return errors.Wrap(errors.ErrInvalidSpec, "ID is required")
	}

	if len(id) > 128 {
		return errors.Wrap(errors.ErrInvalidSpec, "ID too long (max 128 characters)")
	}

	return nil
}
