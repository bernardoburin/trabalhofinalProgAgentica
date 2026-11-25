// Package regras calcula as horas complementares de um aluno a partir do
// Anexo A e do adendo. É o gabarito do experimento: tudo aqui é determinístico.
package regras

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Data é uma data sem hora, no formato AAAA-MM-DD.
type Data struct{ time.Time }

const formatoData = "2006-01-02"

func NovaData(s string) (Data, error) {
	t, err := time.Parse(formatoData, s)
	if err != nil {
		return Data{}, fmt.Errorf("data inválida %q (use AAAA-MM-DD)", s)
	}
	return Data{t}, nil
}

// DeveData é para testes e dados fixos.
func DeveData(s string) Data {
	d, err := NovaData(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Data) String() string { return d.Format(formatoData) }

func (d Data) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Data) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := NovaData(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// Formas de cálculo de um item.
const (
	CalculoHoras  = "horas"  // horas comprovadas, com teto por unidade (projeto, evento, disciplina)
	CalculoFracao = "fracao" // quantidade é a carga horária; conta valor × CH (0,5 nos cursos, 2 em palestras)
	CalculoFixo   = "fixo"   // valor × quantidade de unidades (dias, meses, semestres, artigos...)
)

// Situações de uma atividade.
const (
	Validada   = "validada"
	Pendente   = "pendente"
	Indeferida = "indeferida"
)

type Categoria struct {
	Codigo string   `json:"codigo"`
	Nome   string   `json:"nome"`
	Teto   *float64 `json:"teto"`
}

type Subcategoria struct {
	Codigo    string   `json:"codigo"`
	Categoria string   `json:"categoria"`
	Nome      string   `json:"nome"`
	Teto      *float64 `json:"teto"`
}

type Item struct {
	Codigo          string   `json:"codigo"`
	Subcategoria    string   `json:"subcategoria"`
	Descricao       string   `json:"descricao"`
	Unidade         string   `json:"unidade"`
	Calculo         string   `json:"calculo"`
	Valor           *float64 `json:"valor,omitempty"`
	TetoPorUnidade  *float64 `json:"teto_por_unidade,omitempty"`
	Teto            *float64 `json:"teto,omitempty"`
	CHMin           *float64 `json:"ch_min,omitempty"`
	CHMax           *float64 `json:"ch_max,omitempty"`
	CHAcimaDe       *float64 `json:"ch_acima_de,omitempty"`
	FatorCoinventor *float64 `json:"fator_coinventor,omitempty"`
	VigenteDesde    *Data    `json:"vigente_desde,omitempty"`
	Fonte           string   `json:"fonte,omitempty"`
	Documentacao    string   `json:"documentacao"`
}

type Regras struct {
	Fonte            string         `json:"fonte"`
	TotalExigido     float64        `json:"total_exigido"`
	MinimoCategorias int            `json:"minimo_categorias"`
	Interpretacoes   []string       `json:"interpretacoes"`
	Categorias       []Categoria    `json:"categorias"`
	Subcategorias    []Subcategoria `json:"subcategorias"`
	Itens            []Item         `json:"itens"`

	cat  map[string]*Categoria
	sub  map[string]*Subcategoria
	item map[string]*Item
}

type Atividade struct {
	ID           int      `json:"id"`
	Item         string   `json:"item"`
	Descricao    string   `json:"descricao"`
	Quantidade   float64  `json:"quantidade"`
	CargaHoraria *float64 `json:"carga_horaria,omitempty"`
	Coinventor   bool     `json:"coinventor,omitempty"`
	Data         Data     `json:"data"`
	Protocolo    *Data    `json:"protocolo,omitempty"`
	Situacao     string   `json:"situacao"`
}

type Aluno struct {
	Matricula  string      `json:"matricula"`
	Nome       string      `json:"nome"`
	Ingresso   Data        `json:"ingresso"`
	Atividades []Atividade `json:"atividades"`
}

// Preparar monta os índices e confere a consistência das regras.
func (r *Regras) Preparar() error {
	r.cat = map[string]*Categoria{}
	r.sub = map[string]*Subcategoria{}
	r.item = map[string]*Item{}
	var erros []string
	for i := range r.Categorias {
		r.cat[r.Categorias[i].Codigo] = &r.Categorias[i]
	}
	for i := range r.Subcategorias {
		s := &r.Subcategorias[i]
		if r.cat[s.Categoria] == nil {
			erros = append(erros, fmt.Sprintf("subcategoria %s: categoria %s não existe", s.Codigo, s.Categoria))
		}
		r.sub[s.Codigo] = s
	}
	for i := range r.Itens {
		it := &r.Itens[i]
		if r.sub[it.Subcategoria] == nil {
			erros = append(erros, fmt.Sprintf("item %s: subcategoria %s não existe", it.Codigo, it.Subcategoria))
		}
		switch it.Calculo {
		case CalculoHoras:
		case CalculoFracao, CalculoFixo:
			if it.Valor == nil {
				erros = append(erros, fmt.Sprintf("item %s: cálculo %s precisa de valor", it.Codigo, it.Calculo))
			}
		default:
			erros = append(erros, fmt.Sprintf("item %s: cálculo desconhecido %q", it.Codigo, it.Calculo))
		}
		if _, dup := r.item[it.Codigo]; dup {
			erros = append(erros, fmt.Sprintf("item %s repetido", it.Codigo))
		}
		r.item[it.Codigo] = it
	}
	if r.TotalExigido <= 0 {
		erros = append(erros, "total_exigido precisa ser positivo")
	}
	if len(erros) > 0 {
		return fmt.Errorf("regras inconsistentes: %s", strings.Join(erros, "; "))
	}
	return nil
}

func (r *Regras) Item(codigo string) *Item                 { return r.item[codigo] }
func (r *Regras) Subcategoria(codigo string) *Subcategoria { return r.sub[codigo] }
func (r *Regras) Categoria(codigo string) *Categoria       { return r.cat[codigo] }
func (r *Regras) CategoriaDoItem(it *Item) *Categoria      { return r.cat[r.sub[it.Subcategoria].Categoria] }
