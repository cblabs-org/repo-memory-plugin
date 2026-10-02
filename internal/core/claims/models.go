package claims

import (
	"time"

	"gopkg.in/yaml.v3"
)

type ClaimType string

const (
	TypeInvariant        ClaimType = "invariant"
	TypeDecision         ClaimType = "decision"
	TypeRuledOut         ClaimType = "ruled-out"
	TypeFailureSignature ClaimType = "failure-signature"
	TypeProcedure        ClaimType = "procedure"
	TypeFact             ClaimType = "fact"
	TypeConvention       ClaimType = "convention"
	TypeOpenIssue        ClaimType = "open-issue"
)

type ClaimStatus string

const (
	StatusHypothesis ClaimStatus = "hypothesis"
	StatusObserved   ClaimStatus = "observed"
	StatusVerified   ClaimStatus = "verified"
	StatusMerged     ClaimStatus = "merged"
	StatusConfirmed  ClaimStatus = "confirmed"
	StatusRefuted    ClaimStatus = "refuted"
	StatusSuperseded ClaimStatus = "superseded"
	StatusResolved   ClaimStatus = "resolved"
)

type Anchor struct {
	Kind string `yaml:"kind"`
	Ref  string `yaml:"ref"`
	FP   string `yaml:"fp,omitempty"`
}

type Evidence struct {
	Kind    string `yaml:"kind"`
	Ref     string `yaml:"ref"`
	Steps   []int  `yaml:"steps,omitempty"`
	Outcome string `yaml:"outcome,omitempty"`
}

type Match struct {
	Any         []string `yaml:"any,omitempty"`
	AllInWindow []string `yaml:"all_in_window,omitempty"`
	Source      []string `yaml:"source,omitempty"`
}

type Condition struct {
	Kind   string `yaml:"kind"`
	Ref    string `yaml:"ref,omitempty"`
	State  string `yaml:"state,omitempty"`
	Target string `yaml:"target,omitempty"`
	Result string `yaml:"result,omitempty"`
}

type ClosesWhen struct {
	Unknown bool        `yaml:"-"`
	Any     []Condition `yaml:"any,omitempty"`
}

func (c *ClosesWhen) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "unknown" {
		c.Unknown = true
		return nil
	}
	type alias ClosesWhen
	var a alias
	if err := value.Decode(&a); err != nil {
		return err
	}
	*c = ClosesWhen(a)
	return nil
}

func (c ClosesWhen) MarshalYAML() (interface{}, error) {
	if c.Unknown {
		return "unknown", nil
	}
	type alias ClosesWhen
	return alias(c), nil
}

type Relations struct {
	Supports    []string `yaml:"supports,omitempty"`
	Contradicts []string `yaml:"contradicts,omitempty"`
	Supersedes  []string `yaml:"supersedes,omitempty"`
	Explains    []string `yaml:"explains,omitempty"`
	Refines     []string `yaml:"refines,omitempty"`
	DependsOn   []string `yaml:"depends_on,omitempty"`
}

type Provenance struct {
	Pipeline        string `yaml:"pipeline,omitempty"`
	ExtractorModel  string `yaml:"extractor_model,omitempty"`
	ReconcilerModel string `yaml:"reconciler_model,omitempty"`
}

type Review struct {
	State string `yaml:"state"`
	By    string `yaml:"by"`
	At    string `yaml:"at"`
}

type Claim struct {
	ID           string            `yaml:"id"`
	Type         ClaimType         `yaml:"type"`
	Title        string            `yaml:"title"`
	RelevantWhen string            `yaml:"relevant_when,omitempty"`
	Apply        string            `yaml:"apply,omitempty"`
	Status       ClaimStatus       `yaml:"status"`
	Flags        []string          `yaml:"flags"`
	Anchors      []Anchor          `yaml:"anchors,omitempty"`
	Conditions   map[string]string `yaml:"conditions,omitempty"`
	Evidence     []Evidence        `yaml:"evidence,omitempty"`
	Match        *Match            `yaml:"match,omitempty"`
	ClosesWhen   *ClosesWhen       `yaml:"closes_when,omitempty"`
	Relations    *Relations        `yaml:"relations,omitempty"`
	Scope        map[string]string `yaml:"scope,omitempty"`
	VerifiedAt   string            `yaml:"verified_at,omitempty"`
	Provenance   *Provenance       `yaml:"provenance,omitempty"`
	Review       *Review           `yaml:"review,omitempty"`
	Created      time.Time         `yaml:"created"`
	Updated      time.Time         `yaml:"updated"`

	Body string `yaml:"-"`
}
