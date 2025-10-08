package dotenv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func Load(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{".env"}
	}

	var errs []error
	for _, path := range paths {
		if err := loadFile(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			errs = append(errs, fmt.Errorf("dotenv: %w", err))
		}
	}

	return errors.Join(errs...)
}

func loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line != "" {
			if err := applyLine(line); err != nil {
				return err
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return nil
}

func applyLine(line string) error {
	if strings.HasPrefix(line, "#") {
		return nil
	}

	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}

	key, value, found := strings.Cut(line, "=")
	if !found {
		return nil
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}

	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		} else if value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
	}

	if _, exists := os.LookupEnv(key); exists {
		return nil
	}

	if err := os.Setenv(key, value); err != nil {
		return fmt.Errorf("set env: %w", err)
	}

	return nil
}
