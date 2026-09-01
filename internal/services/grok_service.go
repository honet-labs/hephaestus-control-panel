package services

import (
	"regexp"
	"strings"
)

type GrokService struct {
	patterns map[string]string
}

func NewGrokService() *GrokService {
	// Standard preset Grok patterns
	p := map[string]string{
		"IP":         `(?:\d{1,3}\.){3}\d{1,3}`,
		"IPV4":       `(?:\d{1,3}\.){3}\d{1,3}`,
		"NUMBER":     `(?:[+-]?(?:[0-9]+(?:\.[0-9]+)?))`,
		"WORD":       `\b\w+\b`,
		"NOTSPACE":   `\S+`,
		"SPACE":      `\s*`,
		"TIMESTAMP":  `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?`,
		"HTTPMETHOD": `(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)`,
		"URIPATH":    `/[A-Za-z0-9_./-]*`,
		"STATUSCODE": `[1-5]\d{2}`,
	}
	return &GrokService{patterns: p}
}

type GrokTestResult struct {
	Matched bool                   `json:"matched"`
	Matches map[string]interface{} `json:"matches"`
	Regex   string                 `json:"regex"`
	Error   string                 `json:"error,omitempty"`
}

func (s *GrokService) TestPattern(pattern, text string) GrokTestResult {
	// Expand Grok syntax: %{PATTERN:NAME} -> (?P<NAME>REGEX)
	grokRe := regexp.MustCompile(`%\{([A-Z0-9_]+)(?::([a-zA-Z0-9_]+))?\}`)
	expandedRegex := grokRe.ReplaceAllStringFunc(pattern, func(match string) string {
		sub := grokRe.FindStringSubmatch(match)
		if len(sub) > 1 {
			pName := sub[1]
			varName := ""
			if len(sub) > 2 {
				varName = sub[2]
			}

			pRegex, ok := s.patterns[pName]
			if !ok {
				pRegex = `\S+`
			}

			if varName != "" {
				return fmt.Sprintf(`(?P<%s>%s)`, varName, pRegex)
			}
			return pRegex
		}
		return match
	})

	re, err := regexp.Compile(expandedRegex)
	if err != nil {
		return GrokTestResult{
			Matched: false,
			Regex:   expandedRegex,
			Error:   fmt.Sprintf("Invalid regex compilation: %v", err),
		}
	}

	match := re.FindStringSubmatch(text)
	if match == nil {
		return GrokTestResult{
			Matched: false,
			Regex:   expandedRegex,
			Matches: make(map[string]interface{}),
		}
	}

	matches := make(map[string]interface{})
	groupNames := re.SubexpNames()
	for i, name := range groupNames {
		if i > 0 && i < len(match) {
			if name != "" {
				matches[name] = match[i]
			} else {
				matches[fmt.Sprintf("group_%d", i)] = match[i]
			}
		}
	}

	return GrokTestResult{
		Matched: true,
		Matches: matches,
		Regex:   expandedRegex,
	}
}

func (s *GrokService) GetPresetPatterns() map[string]string {
	copied := make(map[string]string)
	for k, v := range s.patterns {
		copied[k] = v
	}
	return copied
}

func (s *GrokService) AddCustomPattern(name, regex string) {
	name = strings.ToUpper(name)
	s.patterns[name] = regex
}
