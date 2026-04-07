package utils

import (
	"path/filepath"
	"strings"
)

func IsSRTFile(fileName, mimeType string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == ".srt" {
		return true
	}
	m := strings.ToLower(strings.TrimSpace(mimeType))
	return strings.Contains(m, "subrip") || strings.Contains(m, "x-subrip")
}

func IsVTTFile(fileName, mimeType string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == ".vtt" {
		return true
	}
	m := strings.ToLower(strings.TrimSpace(mimeType))
	return strings.Contains(m, "text/vtt")
}

func SubtitleFilenameVTT(fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if base == "" {
		base = "subtitle"
	}
	return base + ".vtt"
}

func SubtitleMimeType(fileName, existing string) string {
	if strings.TrimSpace(existing) != "" {
		return existing
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".vtt":
		return "text/vtt"
	case ".srt", ".ass", ".ssa", ".sub", ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func SRTToVTT(input []byte) []byte {
	normalized := strings.ReplaceAll(string(input), "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	trimmed := strings.TrimSpace(normalized)
	if strings.HasPrefix(trimmed, "WEBVTT") {
		return []byte(normalized)
	}

	lines := strings.Split(normalized, "\n")
	var builder strings.Builder
	builder.WriteString("WEBVTT\n\n")
	for i, line := range lines {
		if strings.Contains(line, "-->") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		builder.WriteString(line)
		if i < len(lines)-1 {
			builder.WriteByte('\n')
		}
	}
	if builder.Len() == 0 || !strings.HasSuffix(builder.String(), "\n") {
		builder.WriteByte('\n')
	}
	return []byte(builder.String())
}
