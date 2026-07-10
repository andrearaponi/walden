package selfupdate

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// verifyChecksum fetches the release's checksums.txt and verifies the staged
// asset's digest against it. Verification fails closed: a missing checksums
// file, a missing entry, or a digest mismatch aborts the update and removes
// the staged file. There is deliberately no bypass.
func verifyChecksum(client *http.Client, baseURL, tag, asset, stagedPath, actualDigest string) error {
	fail := func(err error) error {
		_ = os.Remove(stagedPath)
		return err
	}

	expected, err := fetchExpectedDigest(client, baseURL, tag, asset)
	if err != nil {
		return fail(err)
	}
	if expected != actualDigest {
		return fail(fmt.Errorf("checksum mismatch for %s: expected %s, actual %s", asset, expected, actualDigest))
	}
	return nil
}

// fetchExpectedDigest downloads checksums.txt for tag and returns the digest
// recorded for asset. Lines follow the sha256sum format: "<digest>  <name>".
func fetchExpectedDigest(client *http.Client, baseURL, tag, asset string) (string, error) {
	checksumsURL := fmt.Sprintf("%s/releases/download/%s/checksums.txt", baseURL, tag)

	resp, err := client.Get(checksumsURL)
	if err != nil {
		return "", fmt.Errorf("fetch checksums.txt for release %s: %w", tag, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release %s does not provide checksums.txt (%s); releases up to v0.4.0 predate checksums and cannot be installed by walden update", tag, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("fetch checksums.txt for release %s: %w", tag, err)
	}

	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksums.txt entry for %s in release %s", asset, tag)
}
