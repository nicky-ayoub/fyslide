package main

import (
	"errors"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/tagging"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFileScanner is a mock implementation of service.FileScanner
type mockFileScanner struct{}

func (mfs *mockFileScanner) Run(dir string, logger scan.LoggerFunc) <-chan scan.FileItem {
	c := make(chan scan.FileItem)
	close(c) // Immediately close for a simple mock
	return c
}

var _ service.FileScanner = (*mockFileScanner)(nil) // Ensure interface satisfaction

func TestNewRootCmd_Structure(t *testing.T) {
	dummyGetServiceAndDB := func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
		// This function won't actually be called for structure testing if PersistentPreRunE is not run.
		// However, to be safe, provide a minimal valid return.
		mockedTagDB := &tagging.TagDB{} // Actual TagDB, not mockTagDB, as service.Service expects it.
		mockedSvc := service.NewService(mockedTagDB, &mockFileScanner{}, logger)
		return mockedSvc, mockedTagDB, nil
	}

	rootCmd := NewRootCmd(dummyGetServiceAndDB)

	assert.NotNil(t, rootCmd)
	assert.Equal(t, "fyslide-cli", rootCmd.Use)
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("db-dir"), "db-dir flag should be defined")

	expectedCommands := []string{"add", "remove", "list", "find-by-tag", "list-all-tags", "replace-tag", "batch-add", "batch-remove", "clean", "add-to-tagged"}
	foundCommands := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		foundCommands[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		assert.True(t, foundCommands[expected], "Expected command '%s' to be present", expected)
	}
}

func TestNewRootCmd_PersistentPreRunE_Success(t *testing.T) {
	// Reset package-level vars for test isolation
	originalSvc := svc
	originalTagDB := tagDB
	defer func() {
		svc = originalSvc
		tagDB = originalTagDB
	}()

	testDir := t.TempDir() // For a real TagDB instance
	realTestTagDB, err := tagging.NewTagDB(testDir, nil)
	require.NoError(t, err, "Failed to create real TagDB for test setup")
	defer realTestTagDB.Close()

	expectedSvc := service.NewService(realTestTagDB, &mockFileScanner{}, cliLogger)

	getSvcAndDBFunc := func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
		// In a real scenario, dbPath would be used. For test, we use testDir's realTestTagDB.
		return expectedSvc, realTestTagDB, nil
	}

	rootCmd := NewRootCmd(getSvcAndDBFunc)
	err = rootCmd.PersistentPreRunE(rootCmd, []string{})

	assert.NoError(t, err)
	assert.Equal(t, expectedSvc, svc, "Package-level svc should be set")
	assert.Equal(t, realTestTagDB, tagDB, "Package-level tagDB should be set")
}

func TestNewRootCmd_PersistentPreRunE_Failure(t *testing.T) {
	expectedErr := errors.New("failed to get service and db")
	getSvcAndDBFunc := func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
		return nil, nil, expectedErr
	}

	rootCmd := NewRootCmd(getSvcAndDBFunc)
	err := rootCmd.PersistentPreRunE(rootCmd, []string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize service and tagDB")
	assert.ErrorIs(t, err, expectedErr, "Error should wrap the original error")
}

func TestNewRootCmd_PersistentPostRun(t *testing.T) {
	// Test case 1: tagDB is not nil and should be closed

	// Simulate PersistentPreRunE having set the package-level tagDB
	originalTagDB := tagDB

	testDir := t.TempDir()
	realTestTagDBForPostRun, err := tagging.NewTagDB(testDir, nil)
	require.NoError(t, err)
	// No defer close here, PersistentPostRun should do it.

	tagDB = realTestTagDBForPostRun // Set the package-level variable

	getSvcAndDBFunc := func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
		// This won't be called if we directly call PersistentPostRun
		return nil, nil, nil
	}
	rootCmd := NewRootCmd(getSvcAndDBFunc)

	// Execute PersistentPostRun
	// Note: PersistentPostRun in main.go closes the package-level 'tagDB'.
	rootCmd.PersistentPostRun(rootCmd, []string{})

	// Verify the database is closed by attempting an operation that would fail on a closed DB,
	// like trying to get all tags.
	_, err = realTestTagDBForPostRun.GetAllTags()
	// DB should be closed; assert that subsequent operations return an error
	assert.Error(t, err, "Attempting a view operation on a closed DB should return an error.")

	// Test case 2: tagDB is nil (should not panic)
	tagDB = nil
	assert.NotPanics(t, func() {
		rootCmd.PersistentPostRun(rootCmd, []string{})
	}, "PersistentPostRun should not panic if tagDB is nil")

	// Restore original package-level var
	tagDB = originalTagDB
}
