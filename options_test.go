package confix

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithValidationAfterInitializing(t *testing.T) {
	var before = "before"
	var after = "after"
	assert.NotEqual(t, before, after)
	cfg := config[string]{
		cfg: &before,
	}
	opt := WithValidation[string](func(cfg *string) error {
		*cfg = after
		return nil
	})
	assert.Equal(t, before, *cfg.cfg)
	err := opt.apply(&cfg)
	assert.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, after, *cfg.cfg)
}

func TestWithWritingConfigToFile(t *testing.T) {
	fpath := path.Join(t.TempDir(), "config.yaml")
	assert.NoFileExists(t, fpath)
	tCfg := &testConfig{
		A: generateRandom(t, 100),
	}
	cfg := config[testConfig]{
		cfg: tCfg,
	}
	opt := WithWritingConfigToFile[testConfig](fpath)
	err := opt.apply(&cfg)
	assert.NoError(t, err)
	if assert.FileExists(t, fpath) {
		require.NoError(t, os.Remove(fpath))
	}
}
