package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestRun_abortsWhenSignatureVerificationFails(t *testing.T) {
	httpmock.ActivateNonDefault(httpClient)
	t.Cleanup(httpmock.DeactivateAndReset)

	asset, platform, archName, linuxSuffix, err := getAssetName()
	require.NoError(t, err)

	tag := "v99.0.0-test"
	httpmock.RegisterResponder("GET", "https://api.github.com/repos/smartcontractkit/cre-cli/releases/latest",
		func(_ *http.Request) (*http.Response, error) {
			body, _ := json.Marshal(releaseInfo{TagName: tag})
			return httpmock.NewBytesResponse(http.StatusOK, body), nil
		},
	)

	archiveBytes := createTestArchiveBytes(t, asset, tag, platform, archName, []byte("#!/bin/sh\necho test\n"))

	downloadURL := "https://github.com/smartcontractkit/cre-cli/releases/download/" + tag + "/" + asset
	httpmock.RegisterResponder("GET", downloadURL,
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewBytesResponse(http.StatusOK, archiveBytes), nil
		},
	)

	if platform == "linux" {
		sigAsset := getSigAssetName(platform, archName, linuxSuffix)
		sigURL := "https://github.com/smartcontractkit/cre-cli/releases/download/" + tag + "/" + sigAsset
		httpmock.RegisterResponder("GET", sigURL,
			func(_ *http.Request) (*http.Response, error) {
				return httpmock.NewBytesResponse(http.StatusOK, []byte("invalid-signature")), nil
			},
		)
	}

	err = Run("version v0.0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "release signature verification failed")
}

func createTestArchiveBytes(t *testing.T, asset, tag, platform, archName string, content []byte) []byte {
	t.Helper()
	binName := "cre_" + tag + "_" + platform + "_" + archName
	if platform == "windows" {
		binName += ".exe"
	}

	if filepath.Ext(asset) == ".zip" {
		buf := &bytes.Buffer{}
		zw := zip.NewWriter(buf)
		w, err := zw.Create(binName)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
		require.NoError(t, zw.Close())
		return buf.Bytes()
	}

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: binName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}
