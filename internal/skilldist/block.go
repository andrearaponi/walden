package skilldist

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

// Marker strings delimiting the Walden block in shared AGENTS.md files.
// They MUST stay byte-identical to the markers historically written by
// setup.sh, so existing installations are recognized and upgradable.
const (
	blockBegin = "# --- BEGIN WALDEN SKILL ---"
	blockEnd   = "# --- END WALDEN SKILL ---"
)

// ErrCorruptBlock reports a BEGIN marker without a matching END marker.
var ErrCorruptBlock = errors.New("corrupt walden skill block")

// upsertBlock writes content as a marker-delimited block in target,
// replacing an existing block in place or appending one when absent.
func upsertBlock(target string, content []byte) (replaced bool, err error) {
	existing, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read %s: %w", target, err)
	}

	updated, replaced, err := spliceBlock(existing, buildBlock(content), target)
	if err != nil {
		return false, err
	}
	if _, err := writeFileAtomic(target, updated); err != nil {
		return false, err
	}
	return replaced, nil
}

func buildBlock(content []byte) []byte {
	var block bytes.Buffer
	block.WriteString(blockBegin + "\n")
	block.Write(content)
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		block.WriteString("\n")
	}
	block.WriteString(blockEnd + "\n")
	return block.Bytes()
}

// spliceBlock returns existing with block replacing the current Walden block,
// or appended (separated by a blank line) when no block is present.
func spliceBlock(existing, block []byte, target string) ([]byte, bool, error) {
	begin, end, err := blockBounds(existing, target)
	if err != nil {
		return nil, false, err
	}

	if begin == -1 {
		out := append([]byte(nil), existing...)
		if len(out) > 0 {
			if !bytes.HasSuffix(out, []byte("\n")) {
				out = append(out, '\n')
			}
			out = append(out, '\n')
		}
		return append(out, block...), false, nil
	}

	out := append([]byte(nil), existing[:begin]...)
	out = append(out, block...)
	out = append(out, existing[end:]...)
	return out, true, nil
}

// blockBounds locates the Walden block in data. It returns (-1, -1, nil)
// when no block exists and ErrCorruptBlock when BEGIN has no matching END.
// end points just past the END marker line's trailing newline when present.
func blockBounds(data []byte, target string) (begin, end int, err error) {
	begin = bytes.Index(data, []byte(blockBegin))
	if begin == -1 {
		return -1, -1, nil
	}
	endMarker := bytes.Index(data[begin:], []byte(blockEnd))
	if endMarker == -1 {
		return -1, -1, fmt.Errorf("%w: %s has a BEGIN marker without a matching END marker", ErrCorruptBlock, target)
	}
	end = begin + endMarker + len(blockEnd)
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return begin, end, nil
}

// blockInterior extracts the content between the marker lines.
func blockInterior(data []byte, target string) (interior []byte, found bool, err error) {
	begin, end, err := blockBounds(data, target)
	if err != nil || begin == -1 {
		return nil, false, err
	}
	start := begin + len(blockBegin)
	if start < len(data) && data[start] == '\n' {
		start++
	}
	stop := bytes.Index(data[begin:end], []byte(blockEnd)) + begin
	return data[start:stop], true, nil
}
