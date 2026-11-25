// Package avaliacao carrega as perguntas de teste, confere o gabarito numérico
// com o motor de regras e roda as perguntas no agente.
package avaliacao

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/agente"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/fonte"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

type SimulacaoEsperada struct {
	Atividade        regras.Atividade `json:"atividade"`
	Ganho            float64          `json:"ganho"`
	ConcluidoDepois  *bool            `json:"concluido_depois,omitempty"`
	CategoriasDepois *int             `json:"categorias_depois,omitempty"`
}

type SaldoEsperado struct {
	Total      float64 `json:"total"`
	Concluido  bool    `json:"concluido"`
	Categorias int     `json:"categorias"`
}

type AtividadeEsperada struct {
	ID            int     `json:"id"`
	Contabilizado float64 `json:"contabilizado"`
}

type Verificacao struct {
	Saldo      *SaldoEsperado      `json:"saldo,omitempty"`
	Atividades []AtividadeEsperada `json:"atividades,omitempty"`
	Simulacoes []SimulacaoEsperada `json:"simulacoes,omitempty"`
}

type Pergunta struct {
	ID               int          `json:"id"`
	Aluno            string       `json:"aluno"`
	Tipo             string       `json:"tipo"`
	Armadilha        string       `json:"armadilha"`
	Pergunta         string       `json:"pergunta"`
	RespostaEsperada string       `json:"resposta_esperada"`
	Base             string       `json:"base"`
	Verificacao      *Verificacao `json:"verificacao,omitempty"`
}

func CarregarPerguntas(caminho string) ([]Pergunta, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	var ps []Pergunta
	if err := json.Unmarshal(b, &ps); err != nil {
		return nil, fmt.Errorf("%s: %w", caminho, err)
	}
	return ps, nil
}

// ConferirGabarito roda cada verificação no motor de regras e devolve as
// divergências. Lista vazia = respostas esperadas batem com o cálculo.
func ConferirGabarito(r *regras.Regras, f fonte.Fonte, ps []Pergunta) []string {
	var erros []string
	for _, p := range ps {
		if p.Verificacao == nil {
			continue
		}
		var a regras.Aluno
		if p.Aluno != "" {
			var err error
			if a, err = f.Aluno(p.Aluno); err != nil {
				erros = append(erros, fmt.Sprintf("pergunta %d: %v", p.ID, err))
				continue
			}
		} else {
			a = regras.Aluno{Matricula: "(sem aluno)", Ingresso: regras.DeveData("2020-01-01")}
		}
		v := p.Verificacao
		s := regras.Calcular(r, a, regras.Opcoes{})
		if v.Saldo != nil {
			if s.Total != v.Saldo.Total || s.Concluido != v.Saldo.Concluido || len(s.CategoriasComHoras) != v.Saldo.Categorias {
				erros = append(erros, fmt.Sprintf("pergunta %d: saldo %v/%v/%d, esperado %v/%v/%d",
					p.ID, s.Total, s.Concluido, len(s.CategoriasComHoras), v.Saldo.Total, v.Saldo.Concluido, v.Saldo.Categorias))
			}
		}
		for _, e := range v.Atividades {
			achou := false
			for _, x := range s.Atividades {
				if x.AtividadeID == e.ID {
					achou = true
					if x.Contabilizado != e.Contabilizado {
						erros = append(erros, fmt.Sprintf("pergunta %d: atividade %d contou %v, esperado %v (%s)",
							p.ID, e.ID, x.Contabilizado, e.Contabilizado, strings.Join(x.Motivos, "; ")))
					}
				}
			}
			if !achou {
				erros = append(erros, fmt.Sprintf("pergunta %d: atividade %d não existe", p.ID, e.ID))
			}
		}
		for i, e := range v.Simulacoes {
			sim := regras.Simular(r, a, e.Atividade, regras.Opcoes{})
			if sim.Ganho != e.Ganho {
				erros = append(erros, fmt.Sprintf("pergunta %d, simulação %d (%s): ganho %v, esperado %v (%s)",
					p.ID, i+1, e.Atividade.Item, sim.Ganho, e.Ganho, strings.Join(sim.Nova.Motivos, "; ")))
			}
			if e.ConcluidoDepois != nil && sim.ConcluidoDepois != *e.ConcluidoDepois {
				erros = append(erros, fmt.Sprintf("pergunta %d, simulação %d: concluído depois %v, esperado %v", p.ID, i+1, sim.ConcluidoDepois, *e.ConcluidoDepois))
			}
			if e.CategoriasDepois != nil && len(sim.CategoriasDepois) != *e.CategoriasDepois {
				erros = append(erros, fmt.Sprintf("pergunta %d, simulação %d: %d categorias depois, esperado %d", p.ID, i+1, len(sim.CategoriasDepois), *e.CategoriasDepois))
			}
		}
	}
	return erros
}

// Registro é uma linha do arquivo de resultados (JSONL).
type Registro struct {
	Pergunta         int              `json:"pergunta"`
	Tipo             string           `json:"tipo"`
	Aluno            string           `json:"aluno"`
	Texto            string           `json:"texto_da_pergunta"`
	RespostaEsperada string           `json:"resposta_esperada"`
	Resultado        agente.Resultado `json:"resultado"`
	Erro             string           `json:"erro,omitempty"`
}

// Rodar faz cada pergunta ao agente, uma por vez, e chama aoTerminar a cada uma.
func Rodar(ctx context.Context, ag *agente.Agente, f fonte.Fonte, ps []Pergunta, aoTerminar func(Registro)) error {
	for _, p := range ps {
		reg := Registro{Pergunta: p.ID, Tipo: p.Tipo, Aluno: p.Aluno, Texto: p.Pergunta, RespostaEsperada: p.RespostaEsperada}
		var al *regras.Aluno
		if p.Aluno != "" {
			a, err := f.Aluno(p.Aluno)
			if err != nil {
				return fmt.Errorf("pergunta %d: %w", p.ID, err)
			}
			al = &a
		}
		res, err := ag.Perguntar(ctx, al, p.Pergunta)
		reg.Resultado = res
		if err != nil {
			reg.Erro = err.Error()
		}
		aoTerminar(reg)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}
