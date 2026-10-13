package logx

import (
	"sort"
	"strings"
)

// Redactor scrubs known secret values out of free text, such as the error
// messages returned by database and Redis drivers.
type Redactor struct {
	r *strings.Replacer
}

// NewRedactor builds a Redactor for the given secrets. Longer needles are
// replaced first so that a full DSN wins over the password inside it.
func NewRedactor(secrets ...Secret) *Redactor {
	var needles []string
	seen := map[string]struct{}{}
	for _, s := range secrets {
		for _, n := range s.needles() {
			if _, dup := seen[n]; dup {
				continue
			}
			seen[n] = struct{}{}
			needles = append(needles, n)
		}
	}
	if len(needles) == 0 {
		return &Redactor{}
	}
	sort.Slice(needles, func(i, j int) bool { return len(needles[i]) > len(needles[j]) })
	pairs := make([]string, 0, 2*len(needles))
	for _, n := range needles {
		pairs = append(pairs, n, Mask)
	}
	return &Redactor{r: strings.NewReplacer(pairs...)}
}

// Redact returns s with every known secret replaced by Mask. A nil Redactor
// is valid and returns s unchanged.
func (r *Redactor) Redact(s string) string {
	if r == nil || r.r == nil {
		return s
	}
	return r.r.Replace(s)
}
