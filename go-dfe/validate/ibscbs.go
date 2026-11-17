package validate

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"
)

// Tabelas de CST e cClassTrib do IBS/CBS (Informe Técnico 2025.002 RT, v1.71, aba cClass e aba CST), vendorizadas em
// tables/ibscbs.json a partir de "Tabela IT 2025.002 v.1.71 - cClassTrib.xlsx" do portal nacional (DFE, tipoArquivo 12).
// Cada classe diz em que modelos pode ser informada (BP-e, BP-e TA, BP-e TM) e se exige ou veda os grupos de doação, estorno
// de crédito e tributação regular; cada CST diz se exige o grupo gIBSCBS, a redução (gRed) e o diferimento (gDif).

//go:embed tables/ibscbs.json
var ibscbsTableJSON []byte

// IBSCBSClass é uma linha da tabela de classificação tributária.
type IBSCBSClass struct {
	CST     string
	Code    string
	Name    string
	Rate    string // tipo de alíquota: Padrão, Sem alíquota, Fixa, Uniforme setorial, Uniforme nacional (referência)
	PRedIBS float64
	PRedCBS float64
	// Regular: o cClassTrib exige o grupo gTribRegular. Estorno: exige o grupo de estorno de crédito.
	Regular bool
	Estorno bool
	// Doacao: 0 veda indDoacao, 1 exige, 2 exige só quando a doação é tributada (Art. 6, 2, I).
	Doacao int
	BPe    bool
	BPeTA  bool
	BPeTM  bool
}

// IBSCBSCST é uma linha da tabela de situação tributária.
type IBSCBSCST struct {
	Code    string
	Name    string
	GIBSCBS bool // exige o grupo gIBSCBS (senão o veda)
	GRed    bool // exige a redução de alíquota (senão veda)
	GDif    bool // exige o diferimento (senão veda)
}

type ibscbsRawClass struct {
	CST     string `json:"cst"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Rate    string `json:"rate"`
	PRedIBS string `json:"pRedIBS"`
	PRedCBS string `json:"pRedCBS"`
	Regular string `json:"regular"`
	Estorno string `json:"estorno"`
	Doacao  string `json:"doacao"`
	TA      string `json:"ta"`
	TM      string `json:"tm"`
	BPe     string `json:"bpe"`
}

type ibscbsRawCST struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	GIBSCBS string `json:"gIBSCBS"`
	GRed    string `json:"gRed"`
	GDif    string `json:"gDif"`
}

var (
	ibscbsOnce    sync.Once
	ibscbsClasses []IBSCBSClass
	ibscbsByCode  map[string]IBSCBSClass
	ibscbsCSTs    map[string]IBSCBSCST
)

func loadIBSCBS() {
	var raw struct {
		Classes []ibscbsRawClass `json:"classes"`
		CSTs    []ibscbsRawCST   `json:"csts"`
	}
	if err := json.Unmarshal(ibscbsTableJSON, &raw); err != nil {
		panic("validate: tables/ibscbs.json: " + err.Error()) // embutido e coberto por teste
	}
	flag := func(s string) bool { return s == "1" }
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	ibscbsByCode = make(map[string]IBSCBSClass, len(raw.Classes))
	for _, c := range raw.Classes {
		doacao, _ := strconv.Atoi(c.Doacao)
		cl := IBSCBSClass{
			CST: c.CST, Code: c.Code, Name: c.Name, Rate: c.Rate, PRedIBS: f(c.PRedIBS), PRedCBS: f(c.PRedCBS),
			Regular: flag(c.Regular), Estorno: flag(c.Estorno), Doacao: doacao, BPe: flag(c.BPe), BPeTA: flag(c.TA), BPeTM: flag(c.TM),
		}
		ibscbsClasses = append(ibscbsClasses, cl)
		ibscbsByCode[cl.Code] = cl
	}
	ibscbsCSTs = make(map[string]IBSCBSCST, len(raw.CSTs))
	for _, c := range raw.CSTs {
		ibscbsCSTs[c.Code] = IBSCBSCST{Code: c.Code, Name: c.Name, GIBSCBS: flag(c.GIBSCBS), GRed: flag(c.GRed), GDif: flag(c.GDif)}
	}
}

// IBSCBSClassByCode devolve a classificação tributária pelo código de 6 dígitos.
func IBSCBSClassByCode(code string) (IBSCBSClass, bool) {
	ibscbsOnce.Do(loadIBSCBS)
	c, ok := ibscbsByCode[code]
	return c, ok
}

// IBSCBSCSTByCode devolve a situação tributária pelo código de 3 dígitos.
func IBSCBSCSTByCode(code string) (IBSCBSCST, bool) {
	ibscbsOnce.Do(loadIBSCBS)
	c, ok := ibscbsCSTs[code]
	return c, ok
}

// IBSCBSClassesTA lista as classificações que o BP-e TA aceita (indBPeTA), na ordem da tabela.
func IBSCBSClassesTA() []IBSCBSClass {
	ibscbsOnce.Do(loadIBSCBS)
	var out []IBSCBSClass
	for _, c := range ibscbsClasses {
		if c.BPeTA {
			out = append(out, c)
		}
	}
	return out
}
