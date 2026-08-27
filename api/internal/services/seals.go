package services

// SealNodes monta a lista de lacres do leiaute. Uma função só porque o nó é o
// mesmo em três lugares: transp/vol/lacres da NF-e, infMDFe/lacres e
// rodo/lacRodo do MDF-e. Lacre vazio não vira nó.
func SealNodes(seals []string) []map[string]any {
	out := make([]map[string]any, 0, len(seals))
	for _, s := range seals {
		if s != "" {
			out = append(out, map[string]any{"nLacre": s})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
