package services

// PruneEmpty remove do dicionário do documento fiscal toda tag ou atributo de texto vazio, e o grupo que
// ficou sem filhos por causa disso. Nenhum leiaute aceita elemento de texto vazio (minLength 1 nos XSDs; a
// SEFAZ responde 225), então um dado ausente deve omitir a tag, não emiti-la vazia. Grupo que já nasceu vazio
// no builder é preservado: pode ser marcador do leiaute.
func PruneEmpty(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return pruneMap(t)
	case []map[string]any:
		out := make([]map[string]any, 0, len(t))
		for _, m := range t {
			out = append(out, pruneMap(m))
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && s == "" {
				continue
			}
			out = append(out, PruneEmpty(x))
		}
		return out
	}
	return v
}

func pruneMap(m map[string]any) map[string]any {
	for k, val := range m {
		switch t := val.(type) {
		case string:
			if t == "" {
				delete(m, k)
			}
		case map[string]any:
			if len(t) == 0 {
				continue
			}
			pruneMap(t)
			if len(t) == 0 {
				delete(m, k)
			}
		case []map[string]any, []any:
			m[k] = PruneEmpty(t)
		}
	}
	return m
}
