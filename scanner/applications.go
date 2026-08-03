package scanner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ApplicationsScanner discovers application bundles under configured user
// application roots. It records every bundle instead of maintaining a list of
// known product names.
type ApplicationsScanner struct {
	roots       []string
	ignorePaths []string
}

func NewApplicationsScanner(roots, ignorePaths []string) *ApplicationsScanner {
	return &ApplicationsScanner{roots: uniquePaths(roots), ignorePaths: uniquePaths(ignorePaths)}
}

func (s *ApplicationsScanner) Name() string { return model.SourceApplications }

func (s *ApplicationsScanner) Scan() ([]model.Tool, error) {
	tools := make([]model.Tool, 0)
	var scanErr error
	seen := make(map[string]struct{})

	for _, root := range s.roots {
		root = filepath.Clean(root)
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			scanErr = joinErrors(scanErr, fmt.Errorf("applications: checking %s: %w", root, err))
			continue
		}

		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !entry.IsDir() || !isApplicationBundle(entry.Name()) {
				if entry.IsDir() && isPathExcluded(path, s.ignorePaths) {
					return fs.SkipDir
				}
				return nil
			}
			if isPathExcluded(path, s.ignorePaths) {
				return fs.SkipDir
			}

			path = filepath.Clean(path)
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				tools = append(tools, model.Tool{
					Name:   entry.Name(),
					Source: model.SourceApplications,
					Path:   path,
				})
			}
			// Application bundles can contain nested helper applications. They
			// are part of the bundle, not separate user-installed applications.
			return fs.SkipDir
		})
		if err != nil {
			scanErr = joinErrors(scanErr, fmt.Errorf("applications: scanning %s: %w", root, err))
		}
	}

	sortToolsByNameAndPath(tools)
	return tools, scanErr
}

func isApplicationBundle(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".app")
}

// readApplicationInfo reads only the conventional top-level Info.plist for a
// bundle. In particular, it does not walk the bundle or inspect nested apps.
func readApplicationInfo(bundlePath string) (model.ApplicationInfo, error) {
	plistPath := filepath.Join(bundlePath, "Contents", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return model.ApplicationInfo{}, fmt.Errorf("applications: reading %s: %w", plistPath, err)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
		data, err = ExecRunner("plutil", "-convert", "xml1", "-o", "-", "--", plistPath)
		if err != nil {
			return model.ApplicationInfo{}, fmt.Errorf("applications: converting %s: %w", plistPath, err)
		}
	}

	root, err := parsePropertyList(data)
	if err != nil {
		return model.ApplicationInfo{}, fmt.Errorf("applications: parsing %s: %w", plistPath, err)
	}

	info := model.ApplicationInfo{
		BundleID:       plistString(root, "CFBundleIdentifier"),
		DisplayVersion: firstPlistString(root, "CFBundleDisplayVersion", "CFBundleGetInfoString", "CFBundleVersion"),
		ShortVersion:   plistString(root, "CFBundleShortVersionString"),
		MinimumOS:      plistString(root, "LSMinimumSystemVersion"),
	}
	info.Architectures = plistArchitectures(root)
	return info, nil
}

type plistValue struct {
	kind  string
	text  string
	dict  plistDictionary
	array []plistValue
}

type plistDictionary map[string]plistValue

func parsePropertyList(data []byte) (plistDictionary, error) {
	var document struct {
		XMLName xml.Name        `xml:"plist"`
		Dict    plistDictionary `xml:"dict"`
	}
	if err := xml.NewDecoder(bytes.NewReader(data)).Decode(&document); err != nil {
		return nil, err
	}
	if document.XMLName.Local != "plist" {
		return nil, fmt.Errorf("root element is %q, want plist", document.XMLName.Local)
	}
	if document.Dict == nil {
		return nil, fmt.Errorf("missing plist dictionary")
	}
	return document.Dict, nil
}

func (dict *plistDictionary) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	if start.Name.Local != "dict" {
		return fmt.Errorf("expected dict, got %s", start.Name.Local)
	}

	parsed := make(plistDictionary)
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.EndElement:
			if token.Name == start.Name {
				*dict = parsed
				return nil
			}
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return fmt.Errorf("unexpected text in dict")
			}
		case xml.StartElement:
			if token.Name.Local != "key" {
				return fmt.Errorf("expected key, got %s", token.Name.Local)
			}
			var key string
			if err := decoder.DecodeElement(&key, &token); err != nil {
				return err
			}
			valueStart, err := nextPlistStart(decoder)
			if err != nil {
				return err
			}
			value, err := decodePlistValue(decoder, valueStart)
			if err != nil {
				return fmt.Errorf("key %q: %w", key, err)
			}
			parsed[strings.TrimSpace(key)] = value
		}
	}
}

func (value *plistValue) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	parsed, err := decodePlistValue(decoder, start)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

func decodePlistValue(decoder *xml.Decoder, start xml.StartElement) (plistValue, error) {
	value := plistValue{kind: start.Name.Local}
	switch start.Name.Local {
	case "dict":
		if err := decoder.DecodeElement(&value.dict, &start); err != nil {
			return plistValue{}, err
		}
	case "array":
		var array plistArray
		if err := decoder.DecodeElement(&array, &start); err != nil {
			return plistValue{}, err
		}
		value.array = array
	case "string", "integer", "real", "date", "data", "uid", "true", "false":
		if err := decoder.DecodeElement(&value.text, &start); err != nil {
			return plistValue{}, err
		}
		value.text = strings.TrimSpace(value.text)
	default:
		return plistValue{}, fmt.Errorf("unsupported plist element %q", start.Name.Local)
	}
	return value, nil
}

type plistArray []plistValue

func (array *plistArray) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var parsed plistArray
	for {
		token, err := nextPlistToken(decoder)
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.EndElement:
			if token.Name == start.Name {
				*array = parsed
				return nil
			}
		case xml.StartElement:
			value, err := decodePlistValue(decoder, token)
			if err != nil {
				return err
			}
			parsed = append(parsed, value)
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return fmt.Errorf("unexpected text in array")
			}
		}
	}
}

func nextPlistStart(decoder *xml.Decoder) (xml.StartElement, error) {
	for {
		token, err := nextPlistToken(decoder)
		if err != nil {
			return xml.StartElement{}, err
		}
		if start, ok := token.(xml.StartElement); ok {
			return start, nil
		}
		return xml.StartElement{}, fmt.Errorf("expected plist value")
	}
}

func nextPlistToken(decoder *xml.Decoder) (xml.Token, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if chars, ok := token.(xml.CharData); ok && strings.TrimSpace(string(chars)) == "" {
			continue
		}
		switch token.(type) {
		case xml.Comment, xml.Directive, xml.ProcInst:
			continue
		}
		return token, nil
	}
}

func plistString(dict plistDictionary, key string) string {
	value, ok := dict[key]
	if !ok || value.kind != "string" {
		return ""
	}
	return value.text
}

func firstPlistString(dict plistDictionary, keys ...string) string {
	for _, key := range keys {
		if value := plistString(dict, key); value != "" {
			return value
		}
	}
	return ""
}

func plistArchitectures(dict plistDictionary) []string {
	if value, ok := dict["LSMinimumSystemVersionByArchitecture"]; ok && len(value.dict) > 0 {
		architectures := make([]string, 0, len(value.dict))
		for architecture := range value.dict {
			if architecture != "" {
				architectures = append(architectures, architecture)
			}
		}
		return sortedUniqueStrings(architectures)
	}

	for _, key := range []string{"CFBundleSupportedArchitectures", "CFBundleArchitectures"} {
		value, ok := dict[key]
		if !ok || value.array == nil {
			continue
		}
		architectures := make([]string, 0, len(value.array))
		for _, item := range value.array {
			if item.kind == "string" && item.text != "" {
				architectures = append(architectures, item.text)
			}
		}
		return sortedUniqueStrings(architectures)
	}
	return nil
}

func sortedUniqueStrings(values []string) []string {
	sort.Strings(values)
	unique := values[:0]
	for _, value := range values {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == "." || path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}

func sortToolsByNameAndPath(tools []model.Tool) {
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Name != tools[j].Name {
			return tools[i].Name < tools[j].Name
		}
		return tools[i].Path < tools[j].Path
	})
}
