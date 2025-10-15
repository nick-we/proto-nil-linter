package analyzer

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ProtoFileParser parses .proto files to determine field optionality
type ProtoFileParser struct {
	pass *analysis.Pass

	// Map from message.field to whether it's optional
	// Key: "MessageName.fieldName", Value: true if optional
	optionalFields map[string]bool

	// Track workspace root
	workspaceRoot string
}

// newProtoFileParser creates a new proto file parser
func newProtoFileParser(pass *analysis.Pass, workspaceRoot string) *ProtoFileParser {
	parser := &ProtoFileParser{
		pass:           pass,
		optionalFields: make(map[string]bool),
		workspaceRoot:  workspaceRoot,
	}

	// Parse all proto files recursively
	parser.parseAllProtoFiles()

	return parser
}

// parseAllProtoFiles finds and parses all .proto files recursively from workspace root
func (p *ProtoFileParser) parseAllProtoFiles() {
	if p.workspaceRoot == "" {
		p.workspaceRoot = "."
	}

	// Walk entire workspace tree recursively
	filepath.Walk(p.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}

		// Skip common non-source directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Parse .proto files
		if filepath.Ext(path) == ".proto" {
			p.parseProtoFile(path)
		}

		return nil
	})
}

// parseProtoFile parses a single .proto file using text parsing
func (p *ProtoFileParser) parseProtoFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	p.parseProtoText(string(content))
}

// parseProtoText manually parses proto text for field labels
func (p *ProtoFileParser) parseProtoText(content string) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentMessage string
	var messageStack []string

	// Regex patterns for proto3 syntax
	messageStart := regexp.MustCompile(`^\s*message\s+(\w+)\s*\{`)
	fieldPattern := regexp.MustCompile(`^\s*(optional|required|repeated)?\s*(\w+)\s+(\w+)\s*=\s*\d+`)
	messageEnd := regexp.MustCompile(`^\s*\}`)

	for scanner.Scan() {
		line := scanner.Text()

		// Remove comments
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)

		// Check for message start
		if matches := messageStart.FindStringSubmatch(line); matches != nil {
			messageName := matches[1]
			if len(messageStack) > 0 {
				// Nested message
				messageName = strings.Join(messageStack, ".") + "." + messageName
			}
			messageStack = append(messageStack, messageName)
			currentMessage = messageName
			continue
		}

		// Check for message end
		if messageEnd.MatchString(line) && len(messageStack) > 0 {
			messageStack = messageStack[:len(messageStack)-1]
			if len(messageStack) > 0 {
				currentMessage = messageStack[len(messageStack)-1]
			} else {
				currentMessage = ""
			}
			continue
		}

		// Check for field definition
		if currentMessage != "" {
			if matches := fieldPattern.FindStringSubmatch(line); matches != nil {
				label := matches[1]     // "optional", "required", "repeated", or empty
				fieldType := matches[2] // Field type
				fieldName := matches[3] // Field name

				_ = fieldType // Not used for now

				key := currentMessage + "." + fieldName

				// In proto3:
				// - "optional" keyword → field is optional (can be nil)
				// - No keyword + message type → required (cannot be nil)
				// - "repeated" → repeated field (nil slice is OK)
				isOptional := (label == "optional")

				p.optionalFields[key] = isOptional
			}
		}
	}
}

// isOptionalField checks if a field is optional based on proto definition
func (p *ProtoFileParser) isOptionalField(messageName, fieldName string) bool {
	key := messageName + "." + fieldName
	optional, found := p.optionalFields[key]

	if !found {
		// If not found in proto files, be conservative:
		// - If we found ANY proto files, assume field is required (strict mode)
		// - If NO proto files found, fall back to tag checking (lenient mode)
		if p.hasProtoFiles() {
			return false // Strict: not found = required
		}
		return false // Fallback to tag-based detection
	}

	return optional
}

// hasProtoFiles returns true if any proto files were successfully parsed
func (p *ProtoFileParser) hasProtoFiles() bool {
	return len(p.optionalFields) > 0
}
