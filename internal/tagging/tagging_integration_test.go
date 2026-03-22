//go:build integration
// +build integration

package tagging_test

import (
	"testing"

	"fyslide/internal/tagging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagDB_ReplaceTag(t *testing.T) {
	dir := t.TempDir()
	db, err := tagging.NewTagDB(dir, nil)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.AddTagsToImage("img1.jpg", []string{"cat", "pet"}))
	require.NoError(t, db.AddTagsToImage("img2.jpg", []string{"cat"}))

	// Replace 'cat' -> 'feline'
	require.NoError(t, db.ReplaceTag("cat", "feline"))

	imgs, err := db.GetImages("feline")
	require.NoError(t, err)
	assert.Contains(t, imgs, "img1.jpg")
	assert.Contains(t, imgs, "img2.jpg")

	oldImgs, err := db.GetImages("cat")
	require.NoError(t, err)
	assert.Len(t, oldImgs, 0)
}

func TestTagDB_RemoveAllTagsAndDeleteOrphan(t *testing.T) {
	dir := t.TempDir()
	db, err := tagging.NewTagDB(dir, nil)
	require.NoError(t, err)
	defer db.Close()

	// 'unique' only used by imgA
	require.NoError(t, db.AddTagsToImage("imgA.png", []string{"unique"}))
	require.NoError(t, db.AddTagsToImage("imgB.png", []string{"shared"}))

	// Remove all tags for imgA, making 'unique' orphaned
	require.NoError(t, db.RemoveAllTagsForImage("imgA.png"))

	// Delete orphaned key and confirm it's gone
	require.NoError(t, db.DeleteOrphanedTagKey("unique"))

	tags, err := db.GetAllTags()
	require.NoError(t, err)

	for _, tinfo := range tags {
		assert.NotEqual(t, "unique", tinfo.Name)
	}
}
