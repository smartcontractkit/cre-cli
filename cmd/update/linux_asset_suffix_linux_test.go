//go:build linux

package update

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

func TestParseGlibcVersionFromLddOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name:   "ubuntu 22.04",
			output: "ldd (Ubuntu GLIBC 2.35-0ubuntu3.8) 2.35\nCopyright (C) 2022 Free Software Foundation, Inc.\n",
			want:   "2.35",
		},
		{
			name:   "ubuntu 24.04",
			output: "ldd (Ubuntu GLIBC 2.39-0ubuntu8.4) 2.39\n",
			want:   "2.39",
		},
		{
			name:   "rhel style",
			output: "ldd (GNU libc) 2.34\n",
			want:   "2.34",
		},
		{
			name:    "empty",
			output:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseGlibcVersionFromLddOutput(tt.output)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got.String())
		})
	}
}

func TestLinuxAssetSuffixFromGlibcVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		output string
		want   string
	}{
		{
			output: "ldd (Ubuntu GLIBC 2.35-0ubuntu3.8) 2.35\n",
			want:   linuxLdd235Suffix,
		},
		{
			output: "ldd (Ubuntu GLIBC 2.39-0ubuntu8.4) 2.39\n",
			want:   "",
		},
	}

	threshold, err := semver.NewVersion(linuxGlibcThreshold)
	require.NoError(t, err)

	for _, tt := range tests {
		version, err := parseGlibcVersionFromLddOutput(tt.output)
		require.NoError(t, err)

		suffix := ""
		if version.LessThan(threshold) {
			suffix = linuxLdd235Suffix
		}
		require.Equal(t, tt.want, suffix)
	}
}
