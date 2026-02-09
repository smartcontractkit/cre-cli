package templaterepo

// TemplateMetadata represents the contents of a template.yaml file.
type TemplateMetadata struct {
	Kind        string   `yaml:"kind"`        // "building-block" or "starter-template"
	Name        string   `yaml:"name"`        // Unique slug identifier
	Title       string   `yaml:"title"`       // Human-readable display name
	Description string   `yaml:"description"` // Short description
	Language    string   `yaml:"language"`    // "go" or "typescript"
	Category    string   `yaml:"category"`    // Topic category (e.g., "web3")
	Author      string   `yaml:"author"`
	License     string   `yaml:"license"`
	Tags        []string `yaml:"tags"`      // Searchable tags
	Exclude     []string `yaml:"exclude"`   // Files/dirs to exclude when copying
	Networks    []string `yaml:"networks"`  // Required chain names (e.g., "ethereum-testnet-sepolia")
}

// TemplateSummary is TemplateMetadata plus location info, populated during discovery.
type TemplateSummary struct {
	TemplateMetadata
	Path    string     // Relative path in repo (e.g., "building-blocks/kv-store/kv-store-go")
	Source  RepoSource // Which repo this came from
	BuiltIn bool       // True if this is an embedded built-in template
}

// RepoSource identifies a GitHub repository and ref.
type RepoSource struct {
	Owner string
	Repo  string
	Ref   string // Branch, tag, or SHA
}

// String returns "owner/repo@ref".
func (r RepoSource) String() string {
	return r.Owner + "/" + r.Repo + "@" + r.Ref
}
