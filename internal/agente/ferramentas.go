package agente

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/llm"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

func (ag *Agente) ferramentas() []llm.Ferramenta {
	var codigos []string
	for _, it := range ag.Regras.Itens {
		codigos = append(codigos, it.Codigo)
	}
	return []llm.Ferramenta{
		{
			Nome:      "consultar_saldo",
			Descricao: "Calcula, com as regras oficiais, quantas horas o aluno já tem: total, quanto falta, uso e limite restante de cada categoria e subcategoria, e quanto cada atividade contou e por quê.",
			Esquema:   map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Nome:      "simular_inclusao",
			Descricao: "Simula incluir uma atividade nova e devolve quantas horas ela renderia de fato, depois dos limites de item, subcategoria e categoria, e se o aluno fecharia as 200h. Use uma chamada por alternativa para comparar opções.",
			Esquema: map[string]any{
				"type":     "object",
				"required": []string{"item", "quantidade"},
				"properties": map[string]any{
					"item":            map[string]any{"type": "string", "enum": codigos, "description": "Código do item (lista no fim das instruções)."},
					"quantidade":      map[string]any{"type": "number", "description": "Na unidade do item: horas comprovadas, horas do curso (itens de 50%), dias, meses, semestres ou número de unidades."},
					"carga_horaria":   map[string]any{"type": "number", "description": "Carga horária do curso, quando o item exige mínimo (idiomas: 60h)."},
					"coinventor":      map[string]any{"type": "boolean", "description": "Verdadeiro se o aluno é coinventor (patentes, desenho industrial, marca)."},
					"data_realizacao": map[string]any{"type": "string", "description": "AAAA-MM-DD. Padrão: hoje."},
					"data_protocolo":  map[string]any{"type": "string", "description": "AAAA-MM-DD. Padrão: hoje."},
				},
			},
		},
		{
			Nome:      "detalhar_item",
			Descricao: "Mostra a regra de um item (cálculo, limites, documentação) e quanto o aluno ainda tem livre no item, na subcategoria e na categoria.",
			Esquema: map[string]any{
				"type":       "object",
				"required":   []string{"item"},
				"properties": map[string]any{"item": map[string]any{"type": "string", "enum": codigos}},
			},
		},
	}
}

// executar roda a ferramenta pedida e devolve o texto que volta ao modelo.
// Erros também voltam como texto, para o modelo poder corrigir o pedido.
func (ag *Agente) executar(aluno *regras.Aluno, c llm.Chamada) string {
	a := ag.alunoOuVazio(aluno)
	switch c.Nome {
	case "consultar_saldo":
		return regras.Calcular(ag.Regras, a, regras.Opcoes{}).Texto()

	case "simular_inclusao":
		var p struct {
			Item           string   `json:"item"`
			Quantidade     float64  `json:"quantidade"`
			CargaHoraria   *float64 `json:"carga_horaria"`
			Coinventor     bool     `json:"coinventor"`
			DataRealizacao string   `json:"data_realizacao"`
			DataProtocolo  string   `json:"data_protocolo"`
		}
		if err := json.Unmarshal(argumentos(c), &p); err != nil {
			return "erro: argumentos inválidos: " + err.Error()
		}
		if ag.Regras.Item(p.Item) == nil {
			return fmt.Sprintf("erro: item %q não existe; use um dos códigos da lista", p.Item)
		}
		data, prot := ag.Hoje, ag.Hoje
		var err error
		if p.DataRealizacao != "" {
			if data, err = regras.NovaData(p.DataRealizacao); err != nil {
				return "erro: " + err.Error()
			}
		}
		if p.DataProtocolo != "" {
			if prot, err = regras.NovaData(p.DataProtocolo); err != nil {
				return "erro: " + err.Error()
			}
		}
		nova := regras.Atividade{Item: p.Item, Descricao: "atividade simulada", Quantidade: p.Quantidade,
			CargaHoraria: p.CargaHoraria, Coinventor: p.Coinventor, Data: data, Protocolo: &prot}
		s := regras.Simular(ag.Regras, a, nova, regras.Opcoes{})
		return textoSimulacao(s)

	case "detalhar_item":
		var p struct {
			Item string `json:"item"`
		}
		if err := json.Unmarshal(argumentos(c), &p); err != nil {
			return "erro: argumentos inválidos: " + err.Error()
		}
		return ag.detalhar(a, p.Item)
	}
	return fmt.Sprintf("erro: ferramenta %q não existe", c.Nome)
}

func argumentos(c llm.Chamada) []byte {
	b := []byte(c.Argumentos)
	if len(b) == 0 {
		return []byte("{}")
	}
	// Alguns modelos mandam os argumentos como string JSON.
	var s string
	if json.Unmarshal(b, &s) == nil {
		return []byte(s)
	}
	return b
}

func (ag *Agente) alunoOuVazio(a *regras.Aluno) regras.Aluno {
	if a != nil {
		return *a
	}
	return regras.Aluno{Matricula: "(sem aluno)", Nome: "aluno sem histórico", Ingresso: regras.DeveData("2020-01-01")}
}

func textoSimulacao(s regras.Simulacao) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A atividade simulada valeria %sh brutas e contaria %sh.\n", n(s.Nova.Bruto), n(s.Nova.Contabilizado))
	if len(s.Nova.Motivos) > 0 {
		fmt.Fprintf(&b, "Motivos: %s.\n", strings.Join(s.Nova.Motivos, "; "))
	}
	fmt.Fprintf(&b, "Total: %sh antes, %sh depois (ganho %sh).\n", n(s.TotalAntes), n(s.TotalDepois), n(s.Ganho))
	fmt.Fprintf(&b, "Categorias com horas depois: %s. Concluiria as atividades complementares: %v.\n",
		strings.Join(s.CategoriasDepois, ", "), map[bool]string{true: "sim", false: "não"}[s.ConcluidoDepois])
	for _, p := range s.PendenciasDepois {
		fmt.Fprintf(&b, "Pendência depois: %s.\n", p)
	}
	return b.String()
}

func (ag *Agente) detalhar(a regras.Aluno, codigo string) string {
	it := ag.Regras.Item(codigo)
	if it == nil {
		return fmt.Sprintf("erro: item %q não existe", codigo)
	}
	sub := ag.Regras.Subcategoria(it.Subcategoria)
	cat := ag.Regras.Categoria(sub.Categoria)
	s := regras.Calcular(ag.Regras, a, regras.Opcoes{})
	var b strings.Builder
	fmt.Fprintf(&b, "Item %s: %s\nUnidade: %s; cálculo: %s", it.Codigo, it.Descricao, it.Unidade, it.Calculo)
	if it.Valor != nil {
		fmt.Fprintf(&b, " (valor %s)", n(*it.Valor))
	}
	b.WriteString("\n")
	if it.TetoPorUnidade != nil {
		fmt.Fprintf(&b, "Limite por unidade: %sh\n", n(*it.TetoPorUnidade))
	}
	if it.CHMin != nil || it.CHMax != nil || it.CHAcimaDe != nil {
		fmt.Fprintf(&b, "Faixa de carga horária: mín %s, máx %s, acima de %s\n", pn(it.CHMin), pn(it.CHMax), pn(it.CHAcimaDe))
	}
	if it.FatorCoinventor != nil {
		fmt.Fprintf(&b, "Coinventor recebe %s%%\n", n(*it.FatorCoinventor*100))
	}
	if it.VigenteDesde != nil {
		fmt.Fprintf(&b, "Vale para protocolos a partir de %s (adendo)\n", it.VigenteDesde)
	}
	fmt.Fprintf(&b, "Documentação: %s\n", it.Documentacao)
	usoItem := 0.0
	for _, x := range s.Itens {
		if x.Codigo == it.Codigo {
			usoItem = x.Usado
		}
	}
	fmt.Fprintf(&b, "Para este aluno: item usado %sh de %s; ", n(usoItem), pn(it.Teto))
	for _, x := range s.Subcategorias {
		if x.Codigo == sub.Codigo {
			fmt.Fprintf(&b, "subcategoria %s usada %sh de %s; ", sub.Codigo, n(x.Usado), pn(sub.Teto))
		}
	}
	for _, x := range s.Categorias {
		if x.Codigo == cat.Codigo {
			fmt.Fprintf(&b, "categoria %s usada %sh de %s.", cat.Codigo, n(x.Usado), pn(cat.Teto))
		}
	}
	return b.String()
}

func n(x float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", x), "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}

func pn(x *float64) string {
	if x == nil {
		return "sem limite"
	}
	return n(*x) + "h"
}
