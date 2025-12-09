// Package confix provides a configuration management system that supports JSON, TOML, and YAML formats.
// It allows reading configuration from files and environment variables, with support for validation
// and synchronization across multiple configuration files.
package confix

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// encoder interface defines the contract for encoding configuration data to different formats.
// Implementations include JSON, TOML, and YAML encoders.
type encoder interface {
	Encode(interface{}) error
}

var currentDir, _ = os.Executable()

const (
	jsonConfigFileName = "config.json"
	tomlConfigFileName = "config.toml"
	yamlConfigFileName = "config.yaml"
	ymlConfigFileName  = "config.yml"
)

var (
	// DirEnvName is the environment variable name for specifying the configuration directory path
	DirEnvName = "CONFIG_DIR_PATH"
	// FilePathEnvName is the environment variable name for specifying the configuration file path
	FilePathEnvName = "CONFIG_FILE_PATH"
)

// config represents a configuration instance with type parameter T.
type config[T any] struct {
	// paths contains the list of configuration file paths to be processed
	paths []string
	// cfg holds the pointer to the actual configuration structure
	cfg *T
}

// SetConfigDir sets the directory path for configuration files through environment variable.
// The path will be used to look for configuration files with supported extensions.
func SetConfigDir(dir string) error {
	return os.Setenv(DirEnvName, dir)
}

// SetConfigPath sets the specific configuration file path through environment variable.
// This path will be used instead of searching for configuration files in directories.
func SetConfigPath(path string) error {
	return os.Setenv(FilePathEnvName, path)
}

// New initializes and parsing config.
func New[T any](cfg *T, afterInit ...Option[T]) error {
	_, err := newConfig[T](cfg, afterInit...)
	if err != nil {
		return err
	}

	return nil
}

// newConfig initializes a new configuration instance with the provided configuration structure
// and applies any optional functions after initialization.
func newConfig[T any](cfg *T, afterFunc ...Option[T]) (*config[T], error) {
	c := &config[T]{
		cfg:   cfg,
		paths: []string{},
	}

	err := c.getConfigPaths()
	if err != nil {
		return nil, err
	}

	err = c.load()
	if err != nil {
		return nil, err
	}

	for _, f := range afterFunc {
		if err = f.apply(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// encodeToFile encodes the configuration data to the specified file using the appropriate encoder
// based on the file extension.
func (c *config[T]) encodeToFile(f *os.File) error {
	e, err := getEncoderForFile(path.Ext(f.Name()), f)
	if err != nil {
		return err
	}

	if err = e.Encode(c.cfg); err != nil {
		return err
	}
	return nil
}

// getConfigPaths determines the configuration file paths based on environment variables
// and default locations.
func (c *config[T]) getConfigPaths() error {
	switch configPath, configDir := os.Getenv(FilePathEnvName), os.Getenv(DirEnvName); {

	case configPath != "":
		return c.setConfigPathForOneFile(configPath)

	case configDir != "":
		c.paths = getExistingPaths(
			path.Join(configDir, jsonConfigFileName),
			path.Join(configDir, tomlConfigFileName),
			path.Join(configDir, ymlConfigFileName),
			path.Join(configDir, yamlConfigFileName),
		)
		return nil
	default:
		c.paths = getExistingPaths(
			path.Join(currentDir, tomlConfigFileName),
			path.Join(currentDir, jsonConfigFileName),
			path.Join(currentDir, ymlConfigFileName),
			path.Join(currentDir, yamlConfigFileName),
			tomlConfigFileName,
			jsonConfigFileName,
			ymlConfigFileName,
			yamlConfigFileName,
		)
		return nil
	}
}

// processPath reads and decodes the configuration file at the specified path
// using the appropriate decoder based on the file extension.
func (c *config[T]) processPath(p string) error {
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()

	if fi, statErr := f.Stat(); statErr == nil && fi.Size() == 0 {
		return nil
	}

	switch ext := path.Ext(p); ext {
	case ".json":
		dec := json.NewDecoder(f)
		if err = dec.Decode(c.cfg); err != nil {
			return fmt.Errorf("error while decoding json file: %w", err)
		}
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(f)
		if err = dec.Decode(c.cfg); err != nil {
			return fmt.Errorf("error while decoding yaml file: %w", err)
		}
	case ".toml":
		if _, err = toml.NewDecoder(f).Decode(c.cfg); err != nil {
			return fmt.Errorf("error while decoding toml file: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file extension: %s", ext)
	}
	return nil
}

// load processes all configuration file paths and loads their contents
// into the configuration structure.
func (c *config[T]) load() error {
	for _, p := range c.paths {
		if err := c.processPath(p); err != nil {
			return err
		}
	}
	return nil
}

// setConfigPathForOneFile sets a single configuration file path and creates the file
// if it doesn't exist.
func (c *config[T]) setConfigPathForOneFile(configPath string) error {
	if fileExists(configPath) {
		c.paths = []string{configPath}
		return nil
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	_ = f.Close()

	if err = c.writeToFile(f.Name()); err != nil {
		return err
	}
	return nil
}

// writeToFile writes the configuration data to a file at the specified path
// using a temporary file for atomic writes.
func (c *config[T]) writeToFile(fPath string) error {
	f, err := createTempFile("config*" + path.Ext(fPath))
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	if err = c.encodeToFile(f); err != nil {
		return err
	}

	if err = os.Rename(f.Name(), fPath); err != nil {
		log.Printf("ERROR: os.Rename(%q, %q); err=%v", f.Name(), fPath, err)
		return nil
	}

	return nil
}

// writeToFileAsync asynchronously writes configuration data to a file
// and reports any errors through the error channel.
func (c *config[T]) writeToFileAsync(wg *sync.WaitGroup, fPath string, errCh chan<- error) {
	defer wg.Done()
	err := c.writeToFile(fPath)
	if err != nil {
		errCh <- err
	}
}

// writeToFiles concurrently writes configuration data to all configured paths
// and aggregates any errors that occur during the process.
func (c *config[T]) writeToFiles() error {
	wg := sync.WaitGroup{}

	wg.Add(len(c.paths))
	errCh := make(chan error, len(c.paths))

	for _, fPath := range c.paths {
		go c.writeToFileAsync(&wg, fPath, errCh)
	}
	wg.Wait()
	close(errCh)

	var resultErr error

	for err := range errCh {
		resultErr = errors.Join(resultErr, err)
	}

	return resultErr
}

// createTempFile creates a temporary file with the specified extension in the system's temporary directory.
// It returns a pointer to the created file and any error encountered during the creation process.
// The ext parameter should include the file extension with the dot prefix (e.g., ".json", ".yaml").
func createTempFile(ext string) (*os.File, error) {
	f, err := os.CreateTemp(os.TempDir(), ext)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// fileExists checks if a file exists at the specified path and is not a directory.
// It returns true if the file exists and is a regular file, false otherwise.
// The path parameter should be the full path to the file being checked.
func fileExists(path string) bool {
	f, err := os.Stat(path)
	return err == nil && !f.IsDir()
}

// getEncoderForFile returns encoder to io writer based on extension
func getEncoderForFile(ext string, f io.Writer) (encoder, error) {
	switch ext {
	case ".json":
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc, nil
	case ".toml":
		enc := toml.NewEncoder(f)
		return enc, nil
	case ".yaml", ".yml":
		enc := yaml.NewEncoder(f)
		enc.SetIndent(2)
		return enc, nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// getExistingPaths returns a slice of existing file paths from the provided paths
func getExistingPaths(paths ...string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if fileExists(p) {
			result = append(result, p)
		}
	}
	return result
}
