package confix

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func generateRandom(t testing.TB, l int) string {
	t.Helper()
	b := make([]byte, l)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

type testConfig struct {
	A string `config:"a" json:"a" yaml:"a" toml:"a"`
}

var errUnparsable = errors.New("unparsable")

type unparsableConfig struct{}

func (unparsableConfig) UnmarshalJSON([]byte) error   { return errUnparsable }
func (unparsableConfig) MarshalJSON() ([]byte, error) { return nil, errUnparsable }

func TestNew_UnparsableConfig(t *testing.T) {
	cfg, err := createTempFile("config*.json")
	defer func() {
		require.NoError(t, os.Remove(cfg.Name()))
	}()
	require.NoError(t, err)
	require.NoError(t, cfg.Close())
	require.NoError(t, SetConfigPath(cfg.Name()))
	err = New(new(unparsableConfig), WithSyncingConfigToFiles[unparsableConfig]())
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, errUnparsable)
	}
}

func TestFileExists(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), generateRandom(t, 20))
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, os.Remove(f.Name()))
		}()
		assert.True(t, fileExists(f.Name()))
	})
	t.Run("negative: dir", func(t *testing.T) {
		assert.False(t, fileExists(os.TempDir()))
	})
	t.Run("negative: not existing", func(t *testing.T) {
		assert.False(t, fileExists(os.TempDir()+generateRandom(t, 20)))
	})
}

func TestGetExistingPaths(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		var paths []string
		for range 10 {
			f, err := os.CreateTemp(os.TempDir(), generateRandom(t, 20))
			require.NoError(t, err)
			paths = append(paths, f.Name())
		}
		defer func() {
			for _, p := range paths {
				assert.NoError(t, os.Remove(p))
			}
		}()
		got := getExistingPaths(paths...)
		assert.Equal(t, paths, got)
	})
	t.Run("positive: combined", func(t *testing.T) {
		var paths []string
		var existingPaths []string
		for range 10 {
			f, err := os.CreateTemp(os.TempDir(), generateRandom(t, 50))
			require.NoError(t, err)
			paths = append(paths, f.Name())
			existingPaths = append(existingPaths, f.Name())
		}
		for range 10 {
			paths = append(paths, path.Join(os.TempDir(), generateRandom(t, 50)))
		}
		defer func() {
			for _, p := range existingPaths {
				assert.NoError(t, os.Remove(p))
			}
		}()
		got := getExistingPaths(paths...)
		assert.Equal(t, existingPaths, got)
	})
	t.Run("negative", func(t *testing.T) {
		var paths []string
		for range 10 {
			paths = append(paths, path.Join(os.TempDir(), generateRandom(t, 20)))
		}
		got := getExistingPaths(paths...)
		assert.Empty(t, []string{}, got)
	})
	t.Run("positive: 0 paths", func(t *testing.T) {
		got := getExistingPaths()
		assert.Empty(t, got)
	})
}

func TestGetConfigPaths(t *testing.T) {
	t.Run("positive: config dir", func(t *testing.T) {
		dir := os.TempDir()
		require.NoError(t, os.Unsetenv(DirEnvName))
		require.NoError(t, os.Unsetenv(FilePathEnvName))
		require.NoError(t, os.Setenv(DirEnvName, dir))
		pathsExpected := []string{
			path.Join(dir, jsonConfigFileName),
			path.Join(dir, tomlConfigFileName),
			path.Join(dir, ymlConfigFileName),
			path.Join(dir, yamlConfigFileName),
		}
		for _, p := range pathsExpected {
			f, err := os.Create(p)
			require.NoError(t, err)
			require.NoError(t, f.Close())

		}
		defer func() {
			for _, p := range pathsExpected {
				assert.NoError(t, os.Remove(p))
			}
		}()
		cfg := new(config[testConfig])
		err := cfg.getConfigPaths()
		assert.NoError(t, err)
		assert.Equal(t, pathsExpected, cfg.paths)
	})
	t.Run("positive: config file", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), jsonConfigFileName)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		defer func() {
			assert.NoError(t, os.Remove(f.Name()))
		}()

		require.NoError(t, os.Unsetenv(DirEnvName))
		require.NoError(t, os.Unsetenv(FilePathEnvName))
		require.NoError(t, os.Setenv(FilePathEnvName, f.Name()))

		pathsExpected := []string{
			f.Name(),
		}

		cfg := new(config[testConfig])
		err = cfg.getConfigPaths()
		assert.NoError(t, err)
		assert.Equal(t, pathsExpected, cfg.paths)
	})
	t.Run("positive: unset all", func(t *testing.T) {
		require.NoError(t, os.Unsetenv(DirEnvName))
		require.NoError(t, os.Unsetenv(FilePathEnvName))

		pathsExpected := getExistingPaths(
			path.Join(currentDir, tomlConfigFileName),
			path.Join(currentDir, jsonConfigFileName),
			path.Join(currentDir, ymlConfigFileName),
			path.Join(currentDir, yamlConfigFileName),
			tomlConfigFileName,
			jsonConfigFileName,
			ymlConfigFileName,
			yamlConfigFileName,
		)

		cfg := new(config[testConfig])
		err := cfg.getConfigPaths()
		assert.NoError(t, err)
		assert.Equal(t, pathsExpected, cfg.paths)
	})
}

func TestGetEncoderForFile(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), generateRandom(t, 100)+".json")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, f.Close())
		assert.NoError(t, os.Remove(f.Name()))
	}()
	var enc encoder
	enc, err = getEncoderForFile(".json", f)
	assert.NoError(t, err)
	if assert.NotNil(t, enc) {
		assert.IsType(t, enc, &json.Encoder{})
	}
	enc, err = getEncoderForFile(".yml", f)
	assert.NoError(t, err)
	if assert.NotNil(t, enc) {
		assert.IsType(t, enc, &yaml.Encoder{})
	}
	enc, err = getEncoderForFile(".yaml", f)
	assert.NoError(t, err)
	if assert.NotNil(t, enc) {
		assert.IsType(t, enc, &yaml.Encoder{})
	}
	enc, err = getEncoderForFile(".toml", f)
	assert.NoError(t, err)
	if assert.NotNil(t, enc) {
		assert.IsType(t, enc, &toml.Encoder{})
	}
	enc, err = getEncoderForFile(".unknown", f)
	assert.Error(t, err)
}

func TestWriteToFile(t *testing.T) {
	cfg := &testConfig{}
	t.Run("json", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "config.*.json")
		require.NoError(t, err)
		name := f.Name()
		assert.NoError(t, f.Close())
		assert.NoError(t, os.Remove(name))

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		c := &config[testConfig]{
			cfg: cfg,
		}

		err = c.writeToFile(name)
		assert.NoError(t, err)

		defer func() {
			assert.NoError(t, os.Remove(name))
		}()

		readed, err := os.ReadFile(name)
		require.NoError(t, err)

		assert.JSONEq(t, string(data), string(readed))

	})
	t.Run("yaml", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "config.*.yaml")
		require.NoError(t, err)
		name := f.Name()
		assert.NoError(t, f.Close())
		assert.NoError(t, os.Remove(name))

		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		c := &config[testConfig]{
			cfg: cfg,
		}

		err = c.writeToFile(name)
		assert.NoError(t, err)

		defer func() {
			assert.NoError(t, os.Remove(name))
		}()

		readed, err := os.ReadFile(name)
		require.NoError(t, err)

		assert.YAMLEq(t, string(data), string(readed))
	})
	t.Run("toml", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "config.*.toml")
		require.NoError(t, err)
		name := f.Name()
		assert.NoError(t, f.Close())
		assert.NoError(t, os.Remove(name))

		data, err := toml.Marshal(cfg)
		require.NoError(t, err)

		c := &config[testConfig]{
			cfg: cfg,
		}

		err = c.writeToFile(name)
		assert.NoError(t, err)

		defer func() {
			assert.NoError(t, os.Remove(name))
		}()

		readed, err := os.ReadFile(name)
		require.NoError(t, err)

		assert.Equal(t, string(data), string(readed))

	})
	t.Run("yml", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "config.*.yml")
		require.NoError(t, err)
		name := f.Name()
		assert.NoError(t, f.Close())
		assert.NoError(t, os.Remove(name))

		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		c := &config[testConfig]{
			cfg: cfg,
		}

		err = c.writeToFile(name)
		assert.NoError(t, err)

		defer func() {
			assert.NoError(t, os.Remove(name))
		}()

		readed, err := os.ReadFile(name)
		require.NoError(t, err)

		assert.YAMLEq(t, string(data), string(readed))
	})
}

func TestWriteToFiles(t *testing.T) {
	cfg := &testConfig{}
	t.Run("json", func(t *testing.T) {
		var (
			jsonData, yamlData, tomlData                          []byte
			jsonReadData, yamlReadData, tomlReadData, ymlReadData []byte

			err error
			f   *os.File
		)

		f, err = os.CreateTemp(os.TempDir(), "config.*")
		require.NoError(t, err)
		name := f.Name()
		assert.NoError(t, f.Close())
		assert.NoError(t, os.Remove(name))
		paths := []string{
			name + ".json",
			name + ".toml",
			name + ".yml",
			name + ".yaml",
		}

		jsonData, err = json.Marshal(cfg)
		require.NoError(t, err)

		tomlData, err = toml.Marshal(cfg)
		require.NoError(t, err)

		yamlData, err = yaml.Marshal(cfg)
		require.NoError(t, err)

		c := &config[testConfig]{
			cfg:   cfg,
			paths: paths,
		}

		err = c.writeToFile(name)

		err = c.writeToFiles()
		assert.NoError(t, err)

		defer func() {
			for _, p := range paths {
				assert.True(t, fileExists(p))
				assert.NoError(t, os.Remove(p))
			}
		}()

		jsonReadData, err = os.ReadFile(name + ".json")
		require.NoError(t, err)

		yamlReadData, err = os.ReadFile(name + ".yaml")
		require.NoError(t, err)

		ymlReadData, err = os.ReadFile(name + ".yml")
		require.NoError(t, err)

		tomlReadData, err = os.ReadFile(name + ".toml")
		require.NoError(t, err)

		assert.JSONEq(t, string(jsonData), string(jsonReadData))
		assert.YAMLEq(t, string(yamlData), string(yamlReadData))
		assert.YAMLEq(t, string(yamlData), string(ymlReadData))
		assert.Equal(t, string(tomlData), string(tomlReadData))

	})

	t.Run("negative", func(t *testing.T) {
		c := &config[testConfig]{
			cfg:   cfg,
			paths: []string{"unknown"},
		}

		err := c.writeToFiles()
		assert.Error(t, err)
	})
}

func TestNew(t *testing.T) {
	t.Run("default config with no env and file", func(t *testing.T) {
		os.Clearenv()
		cfg := &testConfig{
			A: generateRandom(t, 20),
		}
		err := New(cfg)
		assert.NotNil(t, cfg)
		assert.NoError(t, err)
		expected := cfg
		assert.Equal(t, expected, cfg)
	})
	t.Run("config from file: json", func(t *testing.T) {
		cfg := &testConfig{
			A: generateRandom(t, 20),
		}
		f, err := os.CreateTemp(os.TempDir(), "config*.json")
		require.NoError(t, err)

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		_, err = f.Write(data)
		require.NoError(t, err)
		assert.NoError(t, f.Close())

		require.NoError(t, os.Setenv(FilePathEnvName, f.Name()))

		parsed := &testConfig{}
		err = New(parsed)
		require.NoError(t, err)
		assert.Equal(t, cfg, parsed)
	})
	t.Run("config from file: toml", func(t *testing.T) {
		cfg := &testConfig{}

		f, err := os.CreateTemp(os.TempDir(), "config*.toml")
		require.NoError(t, err)

		data, err := toml.Marshal(cfg)
		require.NoError(t, err)

		_, err = f.Write(data)
		require.NoError(t, err)
		assert.NoError(t, f.Close())

		require.NoError(t, os.Setenv(FilePathEnvName, f.Name()))

		parsed := &testConfig{}

		err = New(parsed)
		require.NoError(t, err)
		assert.Equal(t, cfg, parsed)
	})
	t.Run("config from file: yaml", func(t *testing.T) {
		cfg := &testConfig{
			A: generateRandom(t, 20),
		}
		f, err := os.CreateTemp(os.TempDir(), "config*.yaml")
		require.NoError(t, err)
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		t.Logf("%s", data)
		_, err = f.Write(data)
		require.NoError(t, err)
		assert.NoError(t, f.Close())

		require.NoError(t, os.Setenv(FilePathEnvName, f.Name()))

		parsed := new(testConfig)

		err = New(parsed)
		require.NoError(t, err)
		assert.Equal(t, cfg, parsed)
	})
	t.Run("config from file: yml", func(t *testing.T) {
		cfg := &testConfig{A: generateRandom(t, 20)}
		f, err := os.CreateTemp(os.TempDir(), "config*.yml")
		require.NoError(t, err)
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		t.Logf("%s", data)
		_, err = f.Write(data)
		require.NoError(t, err)
		assert.NoError(t, f.Close())

		require.NoError(t, os.Setenv(FilePathEnvName, f.Name()))

		parsed := new(testConfig)
		err = New(parsed)

		require.NoError(t, err)
		assert.Equal(t, cfg, parsed)
	})
	t.Run("config from unexisted file", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "config*.yml")
		require.NoError(t, err)
		assert.NoError(t, os.Remove(f.Name()))
		os.Clearenv()

		require.NoError(t, os.Setenv(FilePathEnvName, f.Name()))

		assert.NoFileExists(t, f.Name())
		parsed := new(testConfig)
		err = New(parsed)
		require.NoError(t, err)
		assert.Equal(t, &testConfig{}, parsed)
		assert.FileExists(t, f.Name())
	})
	t.Run("config from dir", func(t *testing.T) {
		cfg := &testConfig{
			A: generateRandom(t, 20),
		}
		dir := os.TempDir()
		os.Clearenv()
		f, err := os.Create(path.Join(dir, "config.yml"))
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, os.Remove(f.Name()))
		}()

		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		_, err = f.Write(data)
		require.NoError(t, err)
		assert.NoError(t, f.Close())

		require.NoError(t, os.Setenv(DirEnvName, dir))

		parsed := new(testConfig)
		err = New(parsed)
		require.NoError(t, err)
		assert.Equal(t, cfg, parsed)
	})
}

func TestCreateTempFile(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		f, err := createTempFile(generateRandom(t, 100))
		assert.NotNil(t, f)
		assert.NoError(t, err)
		assert.FileExists(t, f.Name())
		require.NoError(t, os.Remove(f.Name()))
	})
	t.Run("negative", func(t *testing.T) {
		n := generateRandom(t, 100)
		f1, err := createTempFile(n + "\\///")
		assert.Nil(t, f1)
		assert.Error(t, err)
	})
}

func TestWriteToFile_Negative(t *testing.T) {
	c := &config[testConfig]{}

	err := c.writeToFile("\\///.,")
	assert.Error(t, err)
}
