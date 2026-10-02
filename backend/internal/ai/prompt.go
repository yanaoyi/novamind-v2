package ai

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/yanaoyi/novamindv2/backend/prompts"
)

// Prompt 版本化模板（SPEC.md §17）。
type Prompt struct {
	Name    string
	Version string
	Path    string
	Raw     string

	tpl *template.Template
}

// Meta 是模板清单项。
type Meta struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ErrPromptNotFound 表示找不到模板。
var ErrPromptNotFound = errors.New("Prompt 模板不存在")

var promptFilePattern = regexp.MustCompile(`^(?P<name>[a-z0-9_]+)\.(?P<version>v\d+)\.md$`)

// Engine 是 Prompt 引擎。
type Engine struct {
	byName map[string]map[string]*Prompt
	metas  []Meta
}

// NewEngine 从内嵌模板构建引擎。
// 模板语法用 text/template，渲染时若模板引用不存在的字段会直接报错（fail fast）。
func NewEngine() (*Engine, error) {
	engine := &Engine{byName: map[string]map[string]*Prompt{}}

	err := fs.WalkDir(prompts.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		base := path.Base(p)
		matches := promptFilePattern.FindStringSubmatch(base)
		if matches == nil {
			return fmt.Errorf("模板文件名不符合 <name>.<version>.md 约定: %s", p)
		}
		name, version := matches[1], matches[2]

		raw, err := prompts.FS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("读取模板 %s 失败: %w", p, err)
		}
		tpl, err := template.New(name + "." + version).
			Option("missingkey=error").
			Parse(string(raw))
		if err != nil {
			return fmt.Errorf("解析模板 %s 失败: %w", p, err)
		}

		prompt := &Prompt{Name: name, Version: version, Path: p, Raw: string(raw), tpl: tpl}
		if engine.byName[name] == nil {
			engine.byName[name] = map[string]*Prompt{}
		}
		engine.byName[name][version] = prompt
		engine.metas = append(engine.metas, Meta{Name: name, Version: version, Path: p})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(engine.metas, func(i, j int) bool {
		if engine.metas[i].Name != engine.metas[j].Name {
			return engine.metas[i].Name < engine.metas[j].Name
		}
		return versionNumber(engine.metas[i].Version) < versionNumber(engine.metas[j].Version)
	})
	return engine, nil
}

// Get 取模板；version 为空时返回该模板的最新版本。
func (e *Engine) Get(name, version string) (*Prompt, error) {
	versions, ok := e.byName[name]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrPromptNotFound, name)
	}
	if version == "" {
		var latest *Prompt
		best := -1
		for _, p := range versions {
			if n := versionNumber(p.Version); n > best {
				best, latest = n, p
			}
		}
		return latest, nil
	}
	p, ok := versions[version]
	if !ok {
		return nil, fmt.Errorf("%w: %s.%s", ErrPromptNotFound, name, version)
	}
	return p, nil
}

// List 返回全部模板清单。
func (e *Engine) List() []Meta {
	out := make([]Meta, len(e.metas))
	copy(out, e.metas)
	return out
}

// Render 渲染模板。
func (p *Prompt) Render(data any) (string, error) {
	var buf bytes.Buffer
	if err := p.tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("渲染模板 %s 失败: %w", p.Path, err)
	}
	return buf.String(), nil
}

// versionNumber 把 v2 解析成 2，用于挑最新版本。
func versionNumber(version string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(version, "v"))
	if err != nil {
		return -1
	}
	return n
}
