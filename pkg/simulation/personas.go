package simulation

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

var defaultPersonas = []*types.Persona{
	{
		ID:            "agent-explorer-1",
		Name:          "Galileo",
		Role:          types.RoleExplorer,
		ThinkingStyle: types.StyleDivergent,
		RiskTolerance: 0.9,
		Creativity:    0.9,
		Rigor:         0.4,
		Domains:       []string{"physics", "astronomy"},
		Sociability:   0.8,
		Influence:     0.7,
	},
	{
		ID:            "agent-builder-1",
		Name:          "Euclid",
		Role:          types.RoleBuilder,
		ThinkingStyle: types.StyleConvergent,
		RiskTolerance: 0.3,
		Creativity:    0.5,
		Rigor:         0.95,
		Domains:       []string{"mathematics", "geometry"},
		Sociability:   0.4,
		Influence:     0.8,
	},
	{
		ID:            "agent-reviewer-1",
		Name:          "Popper",
		Role:          types.RoleReviewer,
		ThinkingStyle: types.StyleAnalytical,
		RiskTolerance: 0.4,
		Creativity:    0.4,
		Rigor:         0.9,
		Domains:       []string{"philosophy", "methodology"},
		Sociability:   0.5,
		Influence:     0.6,
	},
	{
		ID:            "agent-synthesizer-1",
		Name:          "Darwin",
		Role:          types.RoleSynthesizer,
		ThinkingStyle: types.StyleLateral,
		RiskTolerance: 0.7,
		Creativity:    0.8,
		Rigor:         0.6,
		Domains:       []string{"biology", "evolution"},
		Sociability:   0.6,
		Influence:     0.7,
	},
	{
		ID:            "agent-communicator-1",
		Name:          "Feynman",
		Role:          types.RoleCommunicator,
		ThinkingStyle: types.StyleIntuitive,
		RiskTolerance: 0.6,
		Creativity:    0.85,
		Rigor:         0.7,
		Domains:       []string{"physics", "education"},
		Sociability:   0.95,
		Influence:     0.9,
	},
}

var namePool = []string{
	"Curie", "Newton", "Turing", "Noether", "Maxwell",
	"Faraday", "Raman", "Dirac", "Hubble", "Planck",
	"Gauss", "Euler", "Bohr", "Tesla", "Shannon",
	"Lovelace", "Mendel", "Wegener", "Fermi", "Haldane",
	"Higgs", "Pauli", "Sagan", "Goodall", "Chandrasekhar",
	"Lagrange", "Kepler", "Laplace", "Pasteur", "Watson",
}

var domainPool = []string{
	"physics", "astronomy", "mathematics", "geometry", "philosophy",
	"methodology", "biology", "evolution", "computing", "ai",
	"chemistry", "materials", "neuroscience", "economics", "linguistics",
}

var roleCycle = []types.AgentRole{
	types.RoleExplorer,
	types.RoleBuilder,
	types.RoleReviewer,
	types.RoleSynthesizer,
	types.RoleCommunicator,
}

var styleByRole = map[types.AgentRole][]types.ThinkingStyle{
	types.RoleExplorer:     {types.StyleDivergent, types.StyleIntuitive, types.StyleLateral},
	types.RoleBuilder:      {types.StyleConvergent, types.StyleAnalytical},
	types.RoleReviewer:     {types.StyleAnalytical, types.StyleConvergent},
	types.RoleSynthesizer:  {types.StyleLateral, types.StyleDivergent},
	types.RoleCommunicator: {types.StyleIntuitive, types.StyleDivergent},
}

// DefaultPersonas returns the built-in set of personas.
func DefaultPersonas() []*types.Persona {
	out := make([]*types.Persona, 0, len(defaultPersonas))
	for _, p := range defaultPersonas {
		clone := *p
		out = append(out, &clone)
	}
	return out
}

// GeneratePersonas creates up to count personas, using defaults first then random.
func GeneratePersonas(count int, seed int64) []*types.Persona {
	if count <= 0 {
		return []*types.Persona{}
	}
	personas := DefaultPersonas()
	if count <= len(personas) {
		return personas[:count]
	}

	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))
	roleCounts := map[types.AgentRole]int{}
	for _, p := range personas {
		roleCounts[p.Role]++
	}
	nameIdx := 0

	for len(personas) < count {
		role := roleCycle[len(personas)%len(roleCycle)]
		roleCounts[role]++

		name := "Agent"
		if nameIdx < len(namePool) {
			name = namePool[nameIdx]
			nameIdx++
		} else {
			name = fmt.Sprintf("Agent-%d", len(personas)+1)
		}

		minRisk, maxRisk := roleRiskRange(role)
		minCreativity, maxCreativity := roleCreativityRange(role)
		minRigor, maxRigor := roleRigorRange(role)
		minSociability, maxSociability := roleSociabilityRange(role)
		minInfluence, maxInfluence := roleInfluenceRange(role)

		persona := &types.Persona{
			ID:            fmt.Sprintf("agent-%s-%d", string(role), roleCounts[role]),
			Name:          name,
			Role:          role,
			ThinkingStyle: pickStyle(rng, role),
			RiskTolerance: sampleRange(rng, minRisk, maxRisk),
			Creativity:    sampleRange(rng, minCreativity, maxCreativity),
			Rigor:         sampleRange(rng, minRigor, maxRigor),
			Domains:       pickDomains(rng, role),
			Sociability:   sampleRange(rng, minSociability, maxSociability),
			Influence:     sampleRange(rng, minInfluence, maxInfluence),
		}

		personas = append(personas, persona)
	}

	return personas
}

func pickStyle(rng *rand.Rand, role types.AgentRole) types.ThinkingStyle {
	styles := styleByRole[role]
	if len(styles) == 0 {
		return types.StyleAnalytical
	}
	return styles[rng.Intn(len(styles))]
}

func pickDomains(rng *rand.Rand, role types.AgentRole) []string {
	count := 2
	if role == types.RoleReviewer {
		count = 1
	}
	chosen := map[string]bool{}
	for len(chosen) < count {
		d := domainPool[rng.Intn(len(domainPool))]
		chosen[d] = true
	}

	domains := make([]string, 0, len(chosen))
	for d := range chosen {
		domains = append(domains, d)
	}

	// Ensure role-typical focus appears sometimes
	switch role {
	case types.RoleBuilder:
		domains = ensureDomain(domains, "mathematics")
	case types.RoleExplorer:
		domains = ensureDomain(domains, "physics")
	case types.RoleReviewer:
		domains = ensureDomain(domains, "methodology")
	case types.RoleSynthesizer:
		domains = ensureDomain(domains, "biology")
	case types.RoleCommunicator:
		domains = ensureDomain(domains, "education")
	}

	return domains
}

func ensureDomain(domains []string, d string) []string {
	for _, v := range domains {
		if v == d {
			return domains
		}
	}
	return append(domains, d)
}

func roleRiskRange(role types.AgentRole) (float64, float64) {
	switch role {
	case types.RoleExplorer:
		return 0.7, 0.95
	case types.RoleBuilder:
		return 0.2, 0.45
	case types.RoleReviewer:
		return 0.25, 0.5
	case types.RoleSynthesizer:
		return 0.5, 0.75
	case types.RoleCommunicator:
		return 0.4, 0.7
	default:
		return 0.3, 0.7
	}
}

func roleCreativityRange(role types.AgentRole) (float64, float64) {
	switch role {
	case types.RoleExplorer:
		return 0.75, 0.95
	case types.RoleBuilder:
		return 0.35, 0.6
	case types.RoleReviewer:
		return 0.3, 0.55
	case types.RoleSynthesizer:
		return 0.6, 0.85
	case types.RoleCommunicator:
		return 0.6, 0.85
	default:
		return 0.4, 0.8
	}
}

func roleRigorRange(role types.AgentRole) (float64, float64) {
	switch role {
	case types.RoleExplorer:
		return 0.3, 0.6
	case types.RoleBuilder:
		return 0.8, 0.98
	case types.RoleReviewer:
		return 0.8, 0.98
	case types.RoleSynthesizer:
		return 0.5, 0.75
	case types.RoleCommunicator:
		return 0.5, 0.75
	default:
		return 0.4, 0.9
	}
}

func roleSociabilityRange(role types.AgentRole) (float64, float64) {
	switch role {
	case types.RoleExplorer:
		return 0.5, 0.85
	case types.RoleBuilder:
		return 0.2, 0.5
	case types.RoleReviewer:
		return 0.3, 0.6
	case types.RoleSynthesizer:
		return 0.4, 0.7
	case types.RoleCommunicator:
		return 0.75, 0.98
	default:
		return 0.3, 0.8
	}
}

func roleInfluenceRange(role types.AgentRole) (float64, float64) {
	switch role {
	case types.RoleExplorer:
		return 0.4, 0.7
	case types.RoleBuilder:
		return 0.6, 0.85
	case types.RoleReviewer:
		return 0.5, 0.8
	case types.RoleSynthesizer:
		return 0.45, 0.75
	case types.RoleCommunicator:
		return 0.7, 0.95
	default:
		return 0.4, 0.8
	}
}

func sampleRange(rng *rand.Rand, min, max float64) float64 {
	if max < min {
		min, max = max, min
	}
	return min + rng.Float64()*(max-min)
}

func NormalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}
