package securityaudit

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	binaryLineMinRunes        = 256
	binaryLineGarbledPercent  = 15
	encodedHexBlobMinRunes    = 512
	encodedBase64BlobMinRunes = 1024
)

var (
	pdfBinaryStreamPattern = regexp.MustCompile(`(?s)stream\r?\n.*?\r?\nendstream`)
	dataURIBase64Pattern   = regexp.MustCompile(`data:[A-Za-z0-9.+-]+/[A-Za-z0-9.+-]+(?:;[A-Za-z0-9.=+-]+)*;base64,[A-Za-z0-9+/=_%-]{256,}`)
)

// sanitizeBinaryAndEncodedPayloads removes non-text binary streams (such as raw
// PDF FlateDecode dumps) and high-entropy encoded blobs (data-URI base64, long
// hex/base64 runs) before text safety scanning. Readable text around or after
// stripped blobs is always preserved so attackers cannot hide instructions
// behind binary markers.
func sanitizeBinaryAndEncodedPayloads(text string) string {
	if text == "" {
		return ""
	}
	cleaned := text
	if strings.Contains(cleaned, "%PDF-") && strings.Contains(cleaned, "stream") && strings.Contains(cleaned, "endstream") {
		cleaned = pdfBinaryStreamPattern.ReplaceAllString(cleaned, "[binary pdf stream omitted]")
	}
	if strings.Contains(cleaned, ";base64,") {
		cleaned = dataURIBase64Pattern.ReplaceAllString(cleaned, "[base64 data omitted]")
	}
	if len(cleaned) >= encodedHexBlobMinRunes {
		cleaned = replaceLongEncodedRuns(cleaned, isHexByte, encodedHexBlobMinRunes, looksLikeHighEntropyHex, "[hex blob omitted]")
	}
	if len(cleaned) >= encodedBase64BlobMinRunes {
		cleaned = replaceLongEncodedRuns(cleaned, isBase64Byte, encodedBase64BlobMinRunes, looksLikeHighEntropyBase64, "[base64 blob omitted]")
	}
	if strings.ContainsRune(cleaned, utf8.RuneError) || containsBinaryControlByte(cleaned) {
		cleaned = stripGarbledBinaryLines(cleaned)
	}
	return strings.TrimSpace(cleaned)
}

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isBase64Byte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '='
}

func replaceLongEncodedRuns(text string, matchByte func(byte) bool, minLen int, check func(string) bool, replacement string) string {
	var builder strings.Builder
	last := 0
	i := 0
	n := len(text)
	for i < n {
		if !matchByte(text[i]) {
			i++
			continue
		}
		start := i
		for i < n && matchByte(text[i]) {
			i++
		}
		if i-start >= minLen && check(text[start:i]) {
			if builder.Len() == 0 {
				builder.Grow(len(text))
			}
			builder.WriteString(text[last:start])
			builder.WriteString(replacement)
			last = i
		}
	}
	if last == 0 {
		return text
	}
	builder.WriteString(text[last:])
	return builder.String()
}

func looksLikeHighEntropyHex(value string) bool {
	if len(value) < encodedHexBlobMinRunes {
		return false
	}
	var seen [128]bool
	distinct := 0
	hasDigit := false
	hasLetter := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hasLetter = true
		}
		if c < 128 && !seen[c] {
			seen[c] = true
			distinct++
		}
	}
	return hasDigit && hasLetter && distinct >= 8
}

func looksLikeHighEntropyBase64(value string) bool {
	if len(value) < encodedBase64BlobMinRunes {
		return false
	}
	var seen [128]bool
	distinct := 0
	hasUpper, hasLower, hasDigit := false, false, false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
		if c < 128 && !seen[c] {
			seen[c] = true
			distinct++
		}
	}
	return hasUpper && hasLower && hasDigit && distinct >= 16
}

func containsBinaryControlByte(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < 0x20 && c != '\n' && c != '\r' && c != '\t' {
			return true
		}
	}
	return false
}

func stripGarbledBinaryLines(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	omittedRun := false
	for _, line := range lines {
		if isGarbledBinaryLine(line) {
			if !omittedRun {
				kept = append(kept, "[binary content omitted]")
				omittedRun = true
			}
			continue
		}
		omittedRun = false
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func isGarbledBinaryLine(line string) bool {
	total := 0
	garbled := 0
	for _, r := range line {
		total++
		if r == utf8.RuneError || (r < 0x20 && r != '\r' && r != '\t') {
			garbled++
		}
	}
	if total < binaryLineMinRunes {
		return false
	}
	return garbled*100 >= total*binaryLineGarbledPercent
}
