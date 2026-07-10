package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
)

// assetName builds the release asset name for a platform, mirroring the
// layout produced by the release workflow: walden-{tag}-{os}-{arch}.
func assetName(tag, goos, goarch string) string {
	return fmt.Sprintf("walden-%s-%s-%s", tag, goos, goarch)
}

// downloadAsset streams the release asset for tag into destPath and returns
// the hex SHA-256 digest of the written bytes. The file is created only
// after the server responds successfully, so failed downloads leave nothing
// behind.
func downloadAsset(client *http.Client, baseURL, tag, asset, destPath string) (string, error) {
	assetURL := fmt.Sprintf("%s/releases/download/%s/%s", baseURL, tag, asset)

	resp, err := client.Get(assetURL)
	if err != nil {
		return "", fmt.Errorf("download %s from release %s: %w", asset, tag, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s from release %s: %s (check that the release exists and ships this platform)", asset, tag, resp.Status)
	}

	file, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("stage %s: %w", asset, err)
	}

	hash := sha256.New()
	_, copyErr := io.Copy(file, io.TeeReader(resp.Body, hash))
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("download %s from release %s: %w", asset, tag, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("stage %s: %w", asset, closeErr)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
