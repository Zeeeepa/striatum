// Package installers ports the RFC 0015 skill bundle and RFC 0025 plugin
// bundle install pipelines from Python (src/striatum/skills, src/striatum/plugins)
// to Go. Template files are embedded as Go assets (mirroring go/pkg/webassets)
// so the binary is self-contained; the rendered bundles match the curated
// context tables the Python installer used (verbs, boundaries, artifact kinds).
package installers

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
)

// embedded carries the byte-equivalent copies of the template trees that
// previously lived under src/striatum/skills/templates and
// src/striatum/plugins/templates. The Python __init__.py package-data markers
// are intentionally not embedded (RFC 0078 Gate B drops them).
//
//go:embed templates/skills templates/plugins
var embedded embed.FS

// readSkillTemplate returns the raw bytes of a skills template, rooted at
// the former striatum.skills.templates package (e.g. "claude_code/workflow.md.tmpl").
func readSkillTemplate(relpath string) (string, error) {
	body, err := embedded.ReadFile("templates/skills/" + relpath)
	if err != nil {
		if errFsNotExist(err) {
			return "", fmt.Errorf("skills template not found: %s", relpath)
		}
		return "", err
	}
	return string(body), nil
}

// readPluginTemplate returns the raw bytes of a plugin template, rooted at
// the former striatum.plugins.templates package.
func readPluginTemplate(relpath string) (string, error) {
	body, err := embedded.ReadFile("templates/plugins/" + relpath)
	if err != nil {
		if errFsNotExist(err) {
			return "", fmt.Errorf("plugin template not found: %s", relpath)
		}
		return "", err
	}
	return string(body), nil
}

func errFsNotExist(err error) bool {
	return err == fs.ErrNotExist || (err != nil && err.Error() == "file does not exist")
}

func sha256Hex(data []byte) string {
	if data == nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
