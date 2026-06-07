package formschema

// Condition is a node in the AND/OR conditional tree. A node is either a group
// (Op is "and"/"or" with Rules) or a leaf (Field+Op+Value). Full parsing,
// validation, and evaluation are implemented in Part 2.
type Condition struct {
	Op    string      `json:"op"`
	Rules []Condition `json:"rules,omitempty"`
	Field string      `json:"field,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// validateConditional is replaced with the full implementation in Part 2.
// For now it accepts any tree (permissive) so Task 3 compiles and tests pass.
func validateConditional(_ *Condition, _ map[string]Field, _ int) error {
	return nil
}
