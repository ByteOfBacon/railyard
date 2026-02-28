package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidAssetType(t *testing.T) {
	require.True(t, IsValidAssetType(AssetTypeMap))
	require.True(t, IsValidAssetType(AssetTypeMod))
	require.False(t, IsValidAssetType(AssetType("unknown")))
}
