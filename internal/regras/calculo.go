package regras

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Nivel é o uso de uma categoria, subcategoria ou item.
type Nivel struct {
	Codigo   string   `json:"codigo"`
	Nome     string   `json:"nome"`
	Usado    float64  `json:"usado"`
	Teto     *float64 `json:"teto"`
	Restante *float64 `json:"restante"`
}

// ResultadoAtividade diz quanto uma atividade contou e por quê.
type ResultadoAtividade struct {
	AtividadeID   int      `json:"atividade_id"`
	Item          string   `json:"item"`
	Descricao     string   `json:"descricao"`
	Situacao      string   `json:"situacao"`
	Bruto         float64  `json:"bruto"`
	Contabilizado float64  `json:"contabilizado"`
	Motivos       []string `json:"motivos,omitempty"`
}

type Saldo struct {
	Matricula          string               `json:"matricula"`
	Nome               string               `json:"nome"`
	Total              float64              `json:"total"`
	Exigido            float64              `json:"exigido"`
	Falta              float64              `json:"falta"`
	HorasPendentes     float64              `json:"horas_de_atividades_pendentes"`
	CategoriasComHoras []string             `json:"categorias_com_horas"`
	MinimoCategorias   int                  `json:"minimo_categorias"`
	Concluido          bool                 `json:"concluido"`
	Pendencias         []string             `json:"pendencias,omitempty"`
	Categorias         []Nivel              `json:"categorias"`
	Subcategorias      []Nivel              `json:"subcategorias"`
	Itens              []Nivel              `json:"itens_usados"`
	Atividades         []ResultadoAtividade `json:"atividades"`
}

type Opcoes struct {
	// ApenasValidadas desconsidera atividades pendentes. Por padrão, pendentes
	// contam (para planejar), e o total delas aparece em HorasPendentes.
	ApenasValidadas bool
}

// Calcular aplica as regras às atividades do aluno, em ordem de data:
// cada atividade conta o bruto do item, cortado pelo que resta no teto do
// item, da subcategoria e da categoria, nessa ordem.
func Calcular(r *Regras, a Aluno, o Opcoes) Saldo {
	ativs := append([]Atividade(nil), a.Atividades...)
	sort.SliceStable(ativs, func(i, j int) bool {
		if !ativs[i].Data.Equal(ativs[j].Data.Time) {
			return ativs[i].Data.Before(ativs[j].Data.Time)
		}
		return ativs[i].ID < ativs[j].ID
	})

	usoItem := map[string]float64{}
	usoSub := map[string]float64{}
	usoCat := map[string]float64{}
	s := Saldo{Matricula: a.Matricula, Nome: a.Nome, Exigido: r.TotalExigido, MinimoCategorias: r.MinimoCategorias}

	for _, at := range ativs {
		res := ResultadoAtividade{AtividadeID: at.ID, Item: at.Item, Descricao: at.Descricao, Situacao: at.Situacao}
		it := r.Item(at.Item)
		bruto, motivos, ok := brutoDaAtividade(r, a, at, it, o)
		res.Bruto = arred(bruto)
		res.Motivos = motivos
		if ok {
			sub := r.Subcategoria(it.Subcategoria)
			cat := r.Categoria(sub.Categoria)
			cont := bruto
			cont, res.Motivos = cortar(cont, it.Teto, usoItem[it.Codigo], "item "+it.Codigo, res.Motivos)
			cont, res.Motivos = cortar(cont, sub.Teto, usoSub[sub.Codigo], "subcategoria "+sub.Codigo, res.Motivos)
			cont, res.Motivos = cortar(cont, cat.Teto, usoCat[cat.Codigo], "categoria "+cat.Codigo, res.Motivos)
			usoItem[it.Codigo] += cont
			usoSub[sub.Codigo] += cont
			usoCat[cat.Codigo] += cont
			res.Contabilizado = arred(cont)
			s.Total += cont
			if at.Situacao == Pendente {
				s.HorasPendentes += cont
			}
		}
		s.Atividades = append(s.Atividades, res)
	}

	s.Total = arred(s.Total)
	s.HorasPendentes = arred(s.HorasPendentes)
	s.Falta = arred(math.Max(0, r.TotalExigido-s.Total))
	for _, c := range r.Categorias {
		s.Categorias = append(s.Categorias, nivel(c.Codigo, c.Nome, usoCat[c.Codigo], c.Teto))
		if usoCat[c.Codigo] > 0 {
			s.CategoriasComHoras = append(s.CategoriasComHoras, c.Codigo)
		}
	}
	for _, sc := range r.Subcategorias {
		s.Subcategorias = append(s.Subcategorias, nivel(sc.Codigo, sc.Nome, usoSub[sc.Codigo], sc.Teto))
	}
	for _, it := range r.Itens {
		if usoItem[it.Codigo] > 0 {
			s.Itens = append(s.Itens, nivel(it.Codigo, it.Descricao, usoItem[it.Codigo], it.Teto))
		}
	}
	if s.Total < r.TotalExigido {
		s.Pendencias = append(s.Pendencias, fmt.Sprintf("faltam %sh para as %sh (Art. 8º)", num(s.Falta), num(r.TotalExigido)))
	}
	if len(s.CategoriasComHoras) < r.MinimoCategorias {
		s.Pendencias = append(s.Pendencias, fmt.Sprintf("horas em %d categoria(s); o Art. 9º, parágrafo único, exige no mínimo %d", len(s.CategoriasComHoras), r.MinimoCategorias))
	}
	s.Concluido = len(s.Pendencias) == 0
	return s
}

// brutoDaAtividade devolve quanto a atividade vale antes dos tetos de item,
// subcategoria e categoria. ok=false quando ela não conta nada.
func brutoDaAtividade(r *Regras, a Aluno, at Atividade, it *Item, o Opcoes) (float64, []string, bool) {
	var m []string
	switch {
	case it == nil:
		return 0, []string{fmt.Sprintf("item %q não existe nas regras", at.Item)}, false
	case at.Situacao == Indeferida:
		return 0, []string{"atividade indeferida pela coordenação"}, false
	case at.Situacao == Pendente && o.ApenasValidadas:
		return 0, []string{"pendente de validação (fora do cálculo só de validadas)"}, false
	case at.Data.Before(a.Ingresso.Time):
		return 0, []string{fmt.Sprintf("realizada em %s, antes do ingresso em %s (Art. 13º)", at.Data, a.Ingresso)}, false
	case at.Quantidade <= 0:
		return 0, []string{"quantidade zero ou negativa"}, false
	}
	if it.VigenteDesde != nil {
		prot := at.Data
		if at.Protocolo != nil {
			prot = *at.Protocolo
		}
		if prot.Before(it.VigenteDesde.Time) {
			return 0, []string{fmt.Sprintf("item do adendo: vale para pedidos protocolados a partir de %s; este foi em %s", it.VigenteDesde, prot)}, false
		}
	}

	ch := at.CargaHoraria
	if ch == nil && it.Calculo == CalculoFracao {
		q := at.Quantidade
		ch = &q
	}
	if it.CHMin != nil || it.CHMax != nil || it.CHAcimaDe != nil {
		if ch == nil {
			return 0, []string{"o item exige carga horária mínima e a atividade não informa a carga horária"}, false
		}
		if it.CHMin != nil && *ch < *it.CHMin {
			return 0, []string{fmt.Sprintf("carga horária de %sh abaixo do mínimo de %sh do item", num(*ch), num(*it.CHMin))}, false
		}
		if it.CHMax != nil && *ch > *it.CHMax {
			return 0, []string{fmt.Sprintf("carga horária de %sh acima do máximo de %sh deste item (veja o item da faixa seguinte)", num(*ch), num(*it.CHMax))}, false
		}
		if it.CHAcimaDe != nil && *ch <= *it.CHAcimaDe {
			return 0, []string{fmt.Sprintf("o item exige carga horária acima de %sh; a atividade tem %sh", num(*it.CHAcimaDe), num(*ch))}, false
		}
	}

	var bruto float64
	switch it.Calculo {
	case CalculoHoras:
		bruto = at.Quantidade
	case CalculoFracao, CalculoFixo:
		bruto = *it.Valor * at.Quantidade
	}
	if it.TetoPorUnidade != nil && bruto > *it.TetoPorUnidade {
		m = append(m, fmt.Sprintf("teto de %sh por unidade: %sh viraram %sh", num(*it.TetoPorUnidade), num(bruto), num(*it.TetoPorUnidade)))
		bruto = *it.TetoPorUnidade
	}
	if at.Coinventor {
		if it.FatorCoinventor != nil {
			m = append(m, fmt.Sprintf("coinventor recebe %s%% da pontuação", num(*it.FatorCoinventor*100)))
			bruto *= *it.FatorCoinventor
		} else {
			m = append(m, "o anexo não prevê redução para coinventor neste item; contado integral")
		}
	}
	return bruto, m, true
}

func cortar(valor float64, teto *float64, usado float64, nome string, motivos []string) (float64, []string) {
	if teto == nil {
		return valor, motivos
	}
	resta := math.Max(0, *teto-usado)
	if valor > resta {
		motivos = append(motivos, fmt.Sprintf("teto da %s (%sh) cortou: sobravam %sh", nome, num(*teto), num(resta)))
		return resta, motivos
	}
	return valor, motivos
}

func nivel(cod, nome string, usado float64, teto *float64) Nivel {
	n := Nivel{Codigo: cod, Nome: nome, Usado: arred(usado), Teto: teto}
	if teto != nil {
		r := arred(math.Max(0, *teto-usado))
		n.Restante = &r
	}
	return n
}

// Simulacao compara o saldo antes e depois de incluir uma atividade.
type Simulacao struct {
	Nova             ResultadoAtividade `json:"atividade_simulada"`
	Ganho            float64            `json:"ganho"`
	GanhoUtil        float64            `json:"ganho_ate_o_exigido"`
	TotalAntes       float64            `json:"total_antes"`
	TotalDepois      float64            `json:"total_depois"`
	ConcluidoDepois  bool               `json:"concluido_depois"`
	CategoriasDepois []string           `json:"categorias_com_horas_depois"`
	PendenciasDepois []string           `json:"pendencias_depois,omitempty"`
}

// Simular inclui a atividade nova (validada, se a situação vier vazia) e
// devolve o efeito no total.
func Simular(r *Regras, a Aluno, nova Atividade, o Opcoes) Simulacao {
	antes := Calcular(r, a, o)
	if nova.ID == 0 {
		nova.ID = -1
	}
	if nova.Situacao == "" {
		nova.Situacao = Validada
	}
	b := a
	b.Atividades = append(append([]Atividade(nil), a.Atividades...), nova)
	depois := Calcular(r, b, o)
	var res ResultadoAtividade
	for _, x := range depois.Atividades {
		if x.AtividadeID == nova.ID {
			res = x
		}
	}
	return Simulacao{
		Nova:             res,
		Ganho:            arred(depois.Total - antes.Total),
		GanhoUtil:        arred(math.Min(depois.Total, r.TotalExigido) - math.Min(antes.Total, r.TotalExigido)),
		TotalAntes:       antes.Total,
		TotalDepois:      depois.Total,
		ConcluidoDepois:  depois.Concluido,
		CategoriasDepois: depois.CategoriasComHoras,
		PendenciasDepois: depois.Pendencias,
	}
}

// Texto resume o saldo para humanos (terminal e ferramenta do agente).
func (s Saldo) Texto() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Aluno %s (%s)\n", s.Matricula, s.Nome)
	fmt.Fprintf(&b, "Total: %sh de %sh · falta %sh · concluído: %v\n", num(s.Total), num(s.Exigido), num(s.Falta), sim(s.Concluido))
	if s.HorasPendentes > 0 {
		fmt.Fprintf(&b, "Desse total, %sh vêm de atividades ainda pendentes de validação.\n", num(s.HorasPendentes))
	}
	for _, p := range s.Pendencias {
		fmt.Fprintf(&b, "Pendência: %s\n", p)
	}
	b.WriteString("Categorias:\n")
	for _, c := range s.Categorias {
		fmt.Fprintf(&b, "  %-4s %-24s %6sh%s\n", c.Codigo, c.Nome, num(c.Usado), teto(c))
	}
	b.WriteString("Subcategorias com horas:\n")
	for _, c := range s.Subcategorias {
		if c.Usado > 0 {
			fmt.Fprintf(&b, "  %-7s %6sh%s  %s\n", c.Codigo, num(c.Usado), teto(c), c.Nome)
		}
	}
	b.WriteString("Atividades:\n")
	for _, a := range s.Atividades {
		fmt.Fprintf(&b, "  #%d %s %s: bruto %sh, contou %sh", a.AtividadeID, a.Item, a.Descricao, num(a.Bruto), num(a.Contabilizado))
		if len(a.Motivos) > 0 {
			fmt.Fprintf(&b, " (%s)", strings.Join(a.Motivos, "; "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func teto(n Nivel) string {
	if n.Teto == nil {
		return " (sem teto)"
	}
	return fmt.Sprintf(" de %sh (restam %sh)", num(*n.Teto), num(*n.Restante))
}

func sim(b bool) string {
	if b {
		return "sim"
	}
	return "não"
}

func arred(x float64) float64 { return math.Round(x*100) / 100 }

func num(x float64) string {
	s := fmt.Sprintf("%.2f", arred(x))
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}
