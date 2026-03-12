package symbolgraph

import (
	"fmt"
	"strings"
)

// SymbolGraph is the top-level structure of a .symbols.json file.
type SymbolGraph struct {
	Metadata      Metadata       `json:"metadata"`
	Module        Module         `json:"module"`
	Symbols       []Symbol       `json:"symbols"`
	Relationships []Relationship `json:"relationships"`
}

// Metadata contains generator information.
type Metadata struct {
	FormatVersion FormatVersion `json:"formatVersion"`
	Generator     string        `json:"generator"`
}

// FormatVersion is the semver of the symbol graph format.
type FormatVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

// Module identifies the module the symbol graph belongs to.
type Module struct {
	Name     string   `json:"name"`
	Platform Platform `json:"platform"`
}

// Platform describes the target platform.
type Platform struct {
	Architecture   string `json:"architecture"`
	Vendor         string `json:"vendor"`
	OperatingSystem struct {
		Name string `json:"name"`
	} `json:"operatingSystem"`
}

// Symbol represents a single API symbol.
type Symbol struct {
	Kind                 SymbolKind    `json:"kind"`
	Identifier           Identifier    `json:"identifier"`
	PathComponents       []string      `json:"pathComponents"`
	Names                Names         `json:"names"`
	DeclarationFragments []Fragment    `json:"declarationFragments"`
	AccessLevel          string        `json:"accessLevel"`
	DocComment           *DocComment   `json:"docComment,omitempty"`
	FunctionSignature    *FuncSig      `json:"functionSignature,omitempty"`
	Availability         []Availability `json:"availability,omitempty"`
	SPI                  bool          `json:"spi,omitempty"`
}

// SymbolKind identifies the kind of symbol.
type SymbolKind struct {
	Identifier  string `json:"identifier"`
	DisplayName string `json:"displayName"`
}

// Identifier contains the precise identifier for a symbol.
type Identifier struct {
	Precise   string `json:"precise"`
	Interfaced string `json:"interfaceLanguage"`
}

// Names holds the various name representations.
type Names struct {
	Title    string     `json:"title"`
	Navigator []Fragment `json:"navigator,omitempty"`
	SubHeading []Fragment `json:"subHeading,omitempty"`
}

// Fragment is a piece of a declaration or name.
type Fragment struct {
	Kind     string `json:"kind"`
	Spelling string `json:"spelling"`
	PreciseID string `json:"preciseIdentifier,omitempty"`
}

// DocComment holds documentation text.
type DocComment struct {
	Lines []DocLine `json:"lines"`
}

// DocLine is a single line of documentation.
type DocLine struct {
	Text string `json:"text"`
}

// FuncSig holds function signature information.
type FuncSig struct {
	Returns    []Fragment  `json:"returns"`
	Parameters []FuncParam `json:"parameters"`
}

// FuncParam describes a function parameter.
type FuncParam struct {
	Name               string     `json:"name"`
	DeclarationFragments []Fragment `json:"declarationFragments"`
}

// Availability describes platform availability.
type Availability struct {
	Domain        string  `json:"domain"`
	Introduced    *SemVer `json:"introduced,omitempty"`
	Deprecated    *SemVer `json:"deprecated,omitempty"`
	Unconditional bool    `json:"isUnconditionallyAvailable,omitempty"`
}

// SemVer is a version number.
type SemVer struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch,omitempty"`
}

func (v *SemVer) String() string {
	if v == nil {
		return ""
	}
	if v.Patch > 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// Relationship describes how two symbols relate.
type Relationship struct {
	Kind           string `json:"kind"`
	Source          string `json:"source"`
	Target         string `json:"target"`
	TargetFallback string `json:"targetFallback,omitempty"`
}

// DeclarationString joins declaration fragments into a single string.
func DeclarationString(frags []Fragment) string {
	var b strings.Builder
	for _, f := range frags {
		b.WriteString(f.Spelling)
	}
	return b.String()
}

// DocCommentText extracts the full text from a DocComment.
func DocCommentText(dc *DocComment) string {
	if dc == nil || len(dc.Lines) == 0 {
		return ""
	}
	lines := make([]string, len(dc.Lines))
	for i, l := range dc.Lines {
		lines[i] = l.Text
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
