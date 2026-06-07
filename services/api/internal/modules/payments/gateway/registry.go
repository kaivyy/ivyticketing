package gateway

type Registry struct {
	gws map[string]Gateway
}

func NewRegistry() *Registry {
	return &Registry{gws: make(map[string]Gateway)}
}

func (r *Registry) Register(g Gateway) {
	r.gws[g.Name()] = g
}

func (r *Registry) Get(name string) (Gateway, bool) {
	g, ok := r.gws[name]
	return g, ok
}

func (r *Registry) Has(name string) bool {
	_, ok := r.gws[name]
	return ok
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.gws))
	for n := range r.gws {
		out = append(out, n)
	}
	return out
}
