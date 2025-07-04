package testing

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/PlakarKorp/kloset/caching"
	"github.com/PlakarKorp/kloset/encryption"
	"github.com/PlakarKorp/kloset/hashing"
	"github.com/PlakarKorp/kloset/kcontext"
	"github.com/PlakarKorp/kloset/logging"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/resources"
	"github.com/PlakarKorp/kloset/storage"
	"github.com/PlakarKorp/kloset/versioning"
	"github.com/stretchr/testify/require"
)

func GenerateRepository(t *testing.T, bufout *bytes.Buffer, buferr *bytes.Buffer, passphrase *[]byte) *repository.Repository {
	// init temporary directories
	tmpRepoDirRoot, err := os.MkdirTemp("", "tmp_repo")
	require.NoError(t, err)
	tmpRepoDir := filepath.Join(tmpRepoDirRoot, "repo")
	tmpCacheDir, err := os.MkdirTemp("", "tmp_cache")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpRepoDir)
		os.RemoveAll(tmpCacheDir)
		os.RemoveAll(tmpRepoDirRoot)
	})

	ctx := kcontext.NewKContext()

	ctx.Client = "plakar-test/1.0.0"

	// create a storage

	r, err := storage.New(ctx, map[string]string{"location": "mock://" + tmpRepoDir})
	require.NotNil(t, r)
	require.NoError(t, err)

	config := storage.NewConfiguration()
	config.Compression = nil
	hasher := hashing.GetHasher(hashing.DEFAULT_HASHING_ALGORITHM)

	var key []byte
	if passphrase != nil {
		key, err = encryption.DeriveKey(config.Encryption.KDFParams, *passphrase)
		require.NoError(t, err)

		canary, err := encryption.DeriveCanary(config.Encryption, key)
		require.NoError(t, err)

		config.Encryption.Canary = canary
		hasher = hashing.GetMACHasher(storage.DEFAULT_HASHING_ALGORITHM, key)
	} else {
		config.Encryption = nil
	}
	serialized, err := config.ToBytes()
	require.NoError(t, err)

	wrappedConfigRd, err := storage.Serialize(hasher, resources.RT_CONFIG, versioning.GetCurrentVersion(resources.RT_CONFIG), bytes.NewReader(serialized))
	require.NoError(t, err)

	wrappedConfig, err := io.ReadAll(wrappedConfigRd)
	require.NoError(t, err)

	err = r.Create(ctx, wrappedConfig)
	require.NoError(t, err)

	// open the storage to load the configuration
	serializedConfig, err := r.Open(ctx)
	require.NoError(t, err)

	// create a repository
	ctx.MaxConcurrency = 1
	if bufout != nil && buferr != nil {
		ctx.Stdout = bufout
		ctx.Stderr = buferr
	}
	cache := caching.NewManager(tmpCacheDir)
	ctx.SetCache(cache)

	// Create a new logger
	var logger *logging.Logger
	if bufout == nil || buferr == nil {
		logger = logging.NewLogger(os.Stdout, os.Stderr)
	} else {
		logger = logging.NewLogger(bufout, buferr)
	}
	if bufout != nil && buferr != nil {
		logger.EnableInfo()
	}
	// logger.EnableTrace("all")
	ctx.SetLogger(logger)
	repo, err := repository.New(ctx, key, r, serializedConfig)
	require.NoError(t, err, "creating repository")

	return repo
}
