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
			want, err := semver.NewVersion(tt.want)
			require.NoError(t, err)
			require.True(t, got.Equal(want))
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
			// Ubuntu 22.04 (glibc 2.35) â€” cannot run the default glibc-2.38 binary.
			output: "ldd (Ubuntu GLIBC 2.35-0ubuntu3.8) 2.35\n",
			want:   linuxLdd235Suffix,
		},
		{
			// Debian 12 (glibc 2.36) â€” cannot run the default glibc-2.38 binary.
			output: "ldd (Debian GLIBC 2.36-9+deb12u14) 2.36\n",
			want:   linuxLdd235Suffix,
		},
		{
			// glibc 2.38 â€” the exact minimum required by the default binary.
			output: "ldd (GNU libc) 2.38\n",
			want:   "",
		},
		{
			// Ubuntu 24.04 (glibc 2.39) â€” can run the default binary.
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
