// Package agente monta o prompt, expõe as ferramentas e conduz o laço com o
// modelo. Modo A: só o regulamento e as atividades do aluno no contexto.
// Modo B: o mesmo, mais ferramentas que calculam com o motor de regras.
package agente

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/llm"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
	"github.com/bernardoburin/trabalhofinalProgAgentica/regulamento"
)

const (
	ModoA = "A" // só documento
	ModoB = "B" // documento + ferramentas
)

type Agente struct {
	Provedor  llm.Provedor
	Regras    *regras.Regras
	Modo      string
	Hoje      regras.Data
	MaxPassos int
}

// Passo registra uma chamada de ferramenta e o que ela devolveu.
type Passo struct {
	Ferramenta string          `json:"ferramenta"`
	Argumentos json.RawMessage `json:"argumentos"`
	Resultado  string          `json:"resultado"`
}

type Resultado struct {
	Resposta string        `json:"resposta"`
	Passos   []Passo       `json:"passos,omitempty"`
	Uso      llm.Uso       `json:"uso"`
	Chamadas int           `json:"chamadas_ao_modelo"`
	Duracao  time.Duration `json:"duracao_ns"`
	Provedor string        `json:"provedor"`
	Modelo   string        `json:"modelo"`
	Modo     string        `json:"modo"`
}

// Perguntar responde à pergunta de um aluno (aluno pode ser nil para
// perguntas gerais sobre o regulamento).
func (ag *Agente) Perguntar(ctx context.Context, aluno *regras.Aluno, pergunta string) (Resultado, error) {
	ini := time.Now()
	res := Resultado{Provedor: ag.Provedor.Nome(), Modelo: ag.Provedor.Modelo(), Modo: ag.Modo}
	sistema := ag.prompt(aluno)
	var ferr []llm.Ferramenta
	if ag.Modo == ModoB {
		ferr = ag.ferramentas()
	}
	hist := []llm.Mensagem{{Papel: llm.PapelUsuario, Texto: pergunta}}
	max := ag.MaxPassos
	if max == 0 {
		max = 8
	}
	for i := 0; i < max; i++ {
		r, err := ag.Provedor.Conversar(ctx, sistema, hist, ferr)
		res.Chamadas++
		res.Uso.Entrada += r.Uso.Entrada
		res.Uso.Saida += r.Uso.Saida
		if err != nil {
			res.Duracao = time.Since(ini)
			return res, err
		}
		if len(r.Chamadas) == 0 {
			res.Resposta = strings.TrimSpace(r.Texto)
			res.Duracao = time.Since(ini)
			return res, nil
		}
		hist = append(hist, llm.Mensagem{Papel: llm.PapelAssistente, Texto: r.Texto, Chamadas: r.Chamadas})
		for _, c := range r.Chamadas {
			saida := ag.executar(aluno, c)
			res.Passos = append(res.Passos, Passo{Ferramenta: c.Nome, Argumentos: c.Argumentos, Resultado: saida})
			hist = append(hist, llm.Mensagem{Papel: llm.PapelFerramenta, ChamadaID: c.ID, Nome: c.Nome, Texto: saida})
		}
	}
	res.Duracao = time.Since(ini)
	res.Resposta = "(o agente não chegou a uma resposta final depois de " + fmt.Sprint(max) + " chamadas ao modelo)"
	return res, nil
}

func (ag *Agente) prompt(aluno *regras.Aluno) string {
	var b strings.Builder
	b.WriteString(`Você é o assistente de horas complementares do curso de Bacharelado em Ciência da Computação do IFSul, Câmpus Passo Fundo. Responde a estudantes em português, de forma direta.

Como responder:
- Baseie-se só no regulamento e no adendo transcritos abaixo. Cite o artigo, a subcategoria ou o item do Anexo A que justifica cada afirmação.
- Ao dizer quantas horas algo vale, considere o limite do item, o da subcategoria, o da categoria (IV e V têm limite de 100h), as horas que o aluno já usou em cada um, o total de 200h (Art. 8º) e o mínimo de 2 categorias (Art. 9º).
- Quando o regulamento não resolve a questão (texto ambíguo, contraditório ou caso omisso), diga isso com clareza e indique que a decisão cabe ao colegiado/coordenação (Art. 14º). Não invente regra.
- Termine com uma linha que comece com "Resposta curta:" e resuma a conclusão em uma frase (por exemplo: "conta 10h", "não conta", "depende da coordenação").
`)
	fmt.Fprintf(&b, "\nData de hoje: %s. Se o aluno não disser a data de uma atividade futura, considere que ela será feita e protocolada depois de hoje.\n", ag.Hoje)
	if ag.Modo == ModoB {
		b.WriteString(`
Você tem ferramentas que calculam com as regras oficiais e as atividades do aluno no banco de dados. Para qualquer número de horas (saldo, quanto falta, quanto uma atividade nova rende), use as ferramentas; não faça essas contas de cabeça. Para escolher a melhor opção entre alternativas, simule cada uma. As ferramentas usam os códigos de item listados no fim deste texto.
`)
	}
	b.WriteString("\n=== REGULAMENTO ===\n")
	b.WriteString(regulamento.Texto)
	b.WriteString("\n=== FIM DO REGULAMENTO ===\n")
	if aluno != nil {
		b.WriteString("\n=== ALUNO ===\n")
		fmt.Fprintf(&b, "Matrícula %s, ingresso em %s.\n", aluno.Matricula, aluno.Ingresso)
		b.WriteString("Atividades registradas (dados brutos informados pelo aluno; as horas contabilizadas NÃO estão calculadas aqui):\n")
		for _, a := range aluno.Atividades {
			it := ag.Regras.Item(a.Item)
			desc, unid := a.Item, ""
			if it != nil {
				desc = fmt.Sprintf("subcategoria %s, %s", it.Subcategoria, it.Descricao)
				unid = it.Unidade
			}
			fmt.Fprintf(&b, "- #%d %q: %s; quantidade %v (%s)", a.ID, a.Descricao, desc, a.Quantidade, unid)
			if a.CargaHoraria != nil {
				fmt.Fprintf(&b, "; carga horária do curso %vh", *a.CargaHoraria)
			}
			if a.Coinventor {
				b.WriteString("; como coinventor")
			}
			fmt.Fprintf(&b, "; realizada em %s", a.Data)
			if a.Protocolo != nil {
				fmt.Fprintf(&b, "; protocolada em %s", a.Protocolo)
			}
			fmt.Fprintf(&b, "; situação: %s\n", a.Situacao)
		}
		if len(aluno.Atividades) == 0 {
			b.WriteString("(nenhuma)\n")
		}
		b.WriteString("=== FIM DO ALUNO ===\n")
	}
	if ag.Modo == ModoB {
		b.WriteString("\n=== CÓDIGOS DE ITEM PARA AS FERRAMENTAS ===\n")
		for _, it := range ag.Regras.Itens {
			fmt.Fprintf(&b, "%s: %s (unidade: %s)\n", it.Codigo, it.Descricao, it.Unidade)
		}
	}
	return b.String()
}
