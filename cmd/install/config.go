package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// allowBinary returns text with name listed in the [plugins] allowed_binaries
// value, and whether text changed. Other lines, comments and line endings are
// kept. It only understands a quoted string value; anything else is an error
// so the user's config is never rewritten blindly.
func allowBinary(text, name string) (string, bool, error) {
	eol, nl := "\n", ""
	if strings.Contains(text, "\r\n") {
		eol, nl = "\r\n", "\r"
	}
	lines := strings.Split(text, "\n")

	header := slices.IndexFunc(lines, func(l string) bool {
		return sectionName(l) == "plugins"
	})
	if header < 0 {
		body := text
		if body != "" && !strings.HasSuffix(body, "\n") {
			body += eol
		}
		if body != "" {
			body += eol
		}
		return body + "[plugins]" + eol + `allowed_binaries = "` + name + `"` + eol, true, nil
	}

	end := len(lines)
	for i := header + 1; i < len(lines); i++ {
		if strings.HasPrefix(stripComment(lines[i]), "[") {
			end = i
			break
		}
	}

	for i := header + 1; i < end; i++ {
		key, value, ok := strings.Cut(lines[i], "=")
		if !ok || strings.TrimSpace(key) != "allowed_binaries" {
			continue
		}
		list, err := quotedValue(value)
		if err != nil {
			return "", false, fmt.Errorf("allowed_binaries must be a quoted string to edit automatically; add %s by hand: %w", name, err)
		}
		names := splitList(list)
		if slices.Contains(names, name) {
			return text, false, nil
		}
		names = append(names, name)
		lines[i] = `allowed_binaries = "` + strings.Join(names, ",") + `"` + nl
		return strings.Join(lines, "\n"), true, nil
	}

	lines = slices.Insert(lines, header+1, `allowed_binaries = "`+name+`"`+nl)
	return strings.Join(lines, "\n"), true, nil
}

// sectionName returns the table name of a [table] header line, or "" if the
// line is not a header.
func sectionName(line string) string {
	t := stripComment(line)
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") || strings.HasPrefix(t, "[[") {
		return ""
	}
	return strings.TrimSpace(t[1 : len(t)-1])
}

// stripComment drops a trailing "# ..." and surrounding space. It does not
// understand '#' inside strings, which is fine for table headers.
func stripComment(line string) string {
	before, _, _ := strings.Cut(line, "#")
	return strings.TrimSpace(before)
}

// quotedValue parses a basic TOML string value such as `"yt-dlp,ffmpeg"`.
func quotedValue(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if !strings.HasPrefix(v, `"`) {
		return "", errors.New("value is not a quoted string")
	}
	end := strings.Index(v[1:], `"`)
	if end < 0 {
		return "", errors.New("unterminated string")
	}
	return v[1 : 1+end], nil
}

func splitList(list string) []string {
	var names []string
	for _, n := range strings.Split(list, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names
}
