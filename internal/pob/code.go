// Package pob converts character snapshots to and from Path of Building 2 import
// codes. A PoB2 code is zlib(Deflate)-compressed build XML, base64-encoded and
// made URL-safe (+ -> -, / -> _). The codec here is exact and round-trip tested;
// the XML generation (build.go) is best-effort and validated live by pasting the
// code into PoB2.
package pob

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Encode compresses build XML and returns a URL-safe base64 PoB2 import code.
func Encode(xml []byte) (string, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(xml); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// Decode reverses Encode: it accepts a PoB2 code (URL-safe or standard base64,
// padded or not) and returns the decompressed build XML.
func Decode(code string) ([]byte, error) {
	code = strings.TrimSpace(code)
	raw, err := decodeBase64Lenient(code)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("zlib reader: %w", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("inflate: %w", err)
	}
	return out, nil
}

// decodeBase64Lenient tolerates URL-safe vs standard alphabets and missing
// padding, since codes are copy-pasted by users from various tools.
func decodeBase64Lenient(s string) ([]byte, error) {
	// Normalise to the URL-safe alphabet.
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}
