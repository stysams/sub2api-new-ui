package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamListViewDoesNotIncludeGroupSnapshot(t *testing.T) {
	data, err := json.Marshal(UpstreamListView{ID: 1, Name: "summary", ResourceCount: 2})

	require.NoError(t, err)
	require.Contains(t, string(data), `"resource_count":2`)
	require.NotContains(t, string(data), `"group_snapshot"`)
}
