package ciot

import "time"

// Lifecycle operations.
const (
	OpCancel  = "cancel"
	OpRectify = "rectify"
	OpClose   = "close"
)

// Lifecycle deadlines (spec section 6, [PDF]).
const (
	cancelWindow        = 24 * time.Hour
	rectifyAgregadoDays = 30
	closeAgregadoDays   = 10
)

// LifecycleContext describes the operation being requested on an existing CIOT. Start is DataInicioViagem (for
// TAC-Agregado, the declaration date), End is DataFimViagem; Inspected is "the ANTT already inspected" (B36/B40).
type LifecycleContext struct {
	Op        string
	Tipo      int
	Start     time.Time
	End       time.Time
	Now       time.Time
	Inspected bool
	// Closing data
	CloseDate time.Time
	PesoCarga string
	Viagens   int
}

// LifecycleRule is one deadline/state rule; Violated returns true when the request breaks it.
type LifecycleRule struct {
	ID        string
	ANTT      int
	Op        string
	Condition string
	Violated  func(c LifecycleContext) bool
}

// LifecycleRules is the catalog of the deadline rules of spec section 6.
var LifecycleRules = []LifecycleRule{
	{"B35", 221, OpCancel, "cancel at most 24 h after the trip start", func(c LifecycleContext) bool { return c.Now.After(c.Start.Add(cancelWindow)) }},
	{"B36", 235, OpCancel, "cancel is forbidden after ANTT inspection", func(c LifecycleContext) bool { return c.Inspected }},
	{"B40", 237, OpRectify, "rectify is forbidden after ANTT inspection", func(c LifecycleContext) bool { return c.Inspected }},
	{"B121", 320, OpRectify, "rectify is forbidden in full-load operations", func(c LifecycleContext) bool { return c.Tipo == TipoLotacao }},
	{"B39", 236, OpRectify, "TAC-Agregado rectify at most 30 days after the trip end", func(c LifecycleContext) bool {
		return c.Tipo == TipoAgregado && c.Now.After(c.End.AddDate(0, 0, rectifyAgregadoDays))
	}},
	{"B116", 0, OpClose, "TAC-Agregado closure at most 10 days after the start", func(c LifecycleContext) bool {
		return c.Tipo == TipoAgregado && c.Now.After(c.Start.AddDate(0, 0, closeAgregadoDays))
	}},
	{"B46", 0, OpClose, "closure date must not be before the trip start", func(c LifecycleContext) bool {
		return !c.CloseDate.IsZero() && c.CloseDate.Before(c.Start)
	}},
	{"B65", 0, OpClose, "full-load closure requires PesoCarga", func(c LifecycleContext) bool { return c.Tipo == TipoLotacao && c.PesoCarga == "" }},
	{"B43", 0, OpClose, "TAC-Agregado closure requires the trips", func(c LifecycleContext) bool { return c.Tipo == TipoAgregado && c.Viagens <= 0 }},
}

// ValidateLifecycle runs the rules of c.Op.
func ValidateLifecycle(c LifecycleContext) []Violation {
	if c.Now.IsZero() {
		c.Now = time.Now()
	}
	var out []Violation
	for _, r := range LifecycleRules {
		if r.Op != c.Op || !r.Violated(c) {
			continue
		}
		rule := Rule{ID: r.ID, ANTT: r.ANTT}
		out = append(out, Violation{r.ID, r.ANTT, r.Op, rule.Internal(), r.Condition})
	}
	return out
}
