package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type loader struct {
	missing []string
	invalid []string
}

func newLoader() *loader {
	return &loader{}
}

func (l *loader) required(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		l.missing = append(l.missing, key)
		return ""
	}

	return value
}

func (l *loader) text(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	return value
}

func (l *loader) enum(key, fallback string, allowed ...string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	for _, option := range allowed {
		if option == value {
			return value
		}
	}

	l.invalid = append(
		l.invalid,
		fmt.Sprintf("%s=%q (allowed: %s)", key, value, strings.Join(allowed, ", ")),
	)

	return fallback
}

func (l *loader) integer(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		l.invalid = append(
			l.invalid,
			fmt.Sprintf("%s=%q (expected integer)", key, value),
		)
		return fallback
	}

	return result
}

func (l *loader) integer32(key string, fallback int32) int32 {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		l.invalid = append(
			l.invalid,
			fmt.Sprintf("%s=%q (expected int32)", key, value),
		)
		return fallback
	}

	return int32(result)
}

func (l *loader) boolean(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		l.invalid = append(
			l.invalid,
			fmt.Sprintf("%s=%q (expected boolean: true or false)", key, value),
		)
		return fallback
	}

	return result
}

func (l *loader) duration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		l.invalid = append(
			l.invalid,
			fmt.Sprintf("%s=%q (expected duration such as 10s, 500ms, or 1m)", key, value),
		)
		return fallback
	}

	return result
}

func (l *loader) list(key, fallback string) []string {
	value, ok := os.LookupEnv(key)
	if !ok {
		value = fallback
	}

	parts := strings.Split(value, ",")

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

func (l *loader) err() error {
	if len(l.missing) == 0 && len(l.invalid) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("invalid configuration:")

	if len(l.missing) > 0 {
		b.WriteString("\nmissing:")
		for _, key := range l.missing {
			_, _ = fmt.Fprintf(&b, "\n  - %s", key)
		}
	}

	if len(l.invalid) > 0 {
		b.WriteString("\nmalformed:")
		for _, entry := range l.invalid {
			_, _ = fmt.Fprintf(&b, "\n  - %s", entry)
		}
	}

	return errors.New(b.String())
}
