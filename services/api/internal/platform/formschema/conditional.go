package formschema

const (
	maxDepth     = 3
	maxLeafCount = 20
)

// Condition is a node in the AND/OR conditional tree. A node is either a group
// (Op is "and"/"or" with Rules) or a leaf (Field+Op+Value).
type Condition struct {
	Op    string      `json:"op"`
	Rules []Condition `json:"rules,omitempty"`
	Field string      `json:"field,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

func (c *Condition) isGroup() bool {
	return c.Op == "and" || c.Op == "or"
}

var leafOps = map[string]bool{
	"equals": true, "notEquals": true, "in": true, "notIn": true,
	"gt": true, "gte": true, "lt": true, "lte": true,
}

// validateConditional checks the tree is well-formed, references only known
// fields with a SMALLER display order (acyclic, no forward refs), uses valid
// operators per field type, and respects depth/leaf limits. selfOrder is the
// display order of the field that owns this conditional.
func validateConditional(cond *Condition, byKey map[string]Field, selfOrder int) error {
	if cond == nil {
		return nil
	}
	leaves := 0
	if err := walk(cond, byKey, selfOrder, 1, &leaves); err != nil {
		return err
	}
	return nil
}

func walk(c *Condition, byKey map[string]Field, selfOrder, depth int, leaves *int) error {
	if c.isGroup() {
		if depth > maxDepth {
			return errf("INVALID_CONDITIONAL", "conditional nesting exceeds depth %d", maxDepth)
		}
		if len(c.Rules) == 0 {
			return errf("INVALID_CONDITIONAL", "group %q must have rules", c.Op)
		}
		for i := range c.Rules {
			if err := walk(&c.Rules[i], byKey, selfOrder, depth+1, leaves); err != nil {
				return err
			}
		}
		return nil
	}
	// leaf
	*leaves++
	if *leaves > maxLeafCount {
		return errf("INVALID_CONDITIONAL", "conditional exceeds %d leaf rules", maxLeafCount)
	}
	if !leafOps[c.Op] {
		return errf("INVALID_CONDITIONAL", "unknown operator %q", c.Op)
	}
	ref, ok := byKey[c.Field]
	if !ok {
		return errf("CONDITIONAL_UNKNOWN_FIELD", "conditional references unknown field %q", c.Field)
	}
	if ref.DisplayOrder >= selfOrder {
		return errf("CONDITIONAL_CYCLE", "conditional may only reference earlier fields (%q)", c.Field)
	}
	if (c.Op == "gt" || c.Op == "gte" || c.Op == "lt" || c.Op == "lte") && !ref.Type.isNumeric() {
		return errf("INVALID_CONDITIONAL", "operator %q requires a numeric/date field, got %s", c.Op, ref.Type)
	}
	if c.Op == "in" || c.Op == "notIn" {
		if _, isArr := c.Value.([]any); !isArr {
			// also accept []interface{} from JSON unmarshalling
			if _, isArr2 := c.Value.([]interface{}); !isArr2 {
				return errf("INVALID_CONDITIONAL", "operator %q requires an array value", c.Op)
			}
		}
	}
	return nil
}

// Evaluate returns whether a field with this conditional should be visible given
// the answers. A nil conditional means always visible.
func Evaluate(cond *Condition, answers map[string]any) bool {
	if cond == nil {
		return true
	}
	if cond.isGroup() {
		if cond.Op == "and" {
			for i := range cond.Rules {
				if !Evaluate(&cond.Rules[i], answers) {
					return false
				}
			}
			return true
		}
		// or
		for i := range cond.Rules {
			if Evaluate(&cond.Rules[i], answers) {
				return true
			}
		}
		return false
	}
	return evalLeaf(cond, answers)
}

func evalLeaf(c *Condition, answers map[string]any) bool {
	got, ok := answers[c.Field]
	if !ok {
		return false
	}
	switch c.Op {
	case "equals":
		return equalsVal(got, c.Value)
	case "notEquals":
		return !equalsVal(got, c.Value)
	case "in":
		return inList(got, c.Value)
	case "notIn":
		return !inList(got, c.Value)
	case "gt", "gte", "lt", "lte":
		return compareNum(got, c.Value, c.Op)
	}
	return false
}

func equalsVal(a, b any) bool {
	if an, aok := toFloat(a); aok {
		if bn, bok := toFloat(b); bok {
			return an == bn
		}
	}
	return toStr(a) == toStr(b)
}

func inList(got, list any) bool {
	arr, ok := asArray(list)
	if !ok {
		return false
	}
	for _, item := range arr {
		if equalsVal(got, item) {
			return true
		}
	}
	return false
}

func compareNum(a, b any, op string) bool {
	an, aok := toFloat(a)
	bn, bok := toFloat(b)
	if !aok || !bok {
		return false
	}
	switch op {
	case "gt":
		return an > bn
	case "gte":
		return an >= bn
	case "lt":
		return an < bn
	case "lte":
		return an <= bn
	}
	return false
}

func asArray(v any) ([]any, bool) {
	if a, ok := v.([]any); ok {
		return a, true
	}
	if a, ok := v.([]interface{}); ok {
		return a, true
	}
	return nil, false
}
