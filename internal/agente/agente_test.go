package agente

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/fonte"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/llm"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

// falso pede uma simulação na primeira chamada e responde com o que recebeu.
type falso struct {
	sistemas []string
	ultimo   []llm.Mensagem
}

func (f *falso) Nome() string   { return "falso" }
func (f *falso) Modelo() string { return "falso" }
func (f *falso) Conversar(_ context.Context, sis string, h []llm.Mensagem, ferr []llm.Ferramenta) (llm.Resposta, error) {
	f.sistemas = append(f.sistemas, sis)
	f.ultimo = h
	if len(ferr) > 0 && len(h) == 1 {
		return llm.Resposta{Chamadas: []llm.Chamada{{ID: "1", Nome: "simular_inclusao",
			Argumentos: json.RawMessage(`{"item":"IV-II-1","quantidade":6}`)}}}, nil
	}
	return llm.Resposta{Texto: "ok. Resposta curta: " + h[len(h)-1].Texto}, nil
}

func preparar(t *testing.T, modo string) (*Agente, *falso, regras.Aluno) {
	_, arq, _, _ := runtime.Caller(0)
	f := fonte.JSON{Dir: filepath.Join(filepath.Dir(arq), "..", "..", "dados")}
	r, err := f.Regras()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := f.Aluno("2022101")
	p := &falso{}
	return &Agente{Provedor: p, Regras: r, Modo: modo, Hoje: regras.DeveData("2026-11-25")}, p, a
}

func TestModoBExecutaFerramentaComOMotor(t *testing.T) {
	ag, p, a := preparar(t, ModoB)
	res, err := ag.Perguntar(context.Background(), &a, "6 meses de estágio, quanto dá?")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Passos) != 1 || !strings.Contains(res.Passos[0].Resultado, "contaria 10h") {
		t.Fatalf("passos: %+v", res.Passos)
	}
	if res.Chamadas != 2 || !strings.Contains(res.Resposta, "contaria 10h") {
		t.Errorf("resultado: %+v", res)
	}
	if !strings.Contains(p.sistemas[0], "CÓDIGOS DE ITEM") || !strings.Contains(p.sistemas[0], "Art. 13º") {
		t.Error("prompt do modo B sem catálogo ou sem regulamento")
	}
}

func TestModoANaoTemFerramentasNemContas(t *testing.T) {
	ag, p, a := preparar(t, ModoA)
	res, err := ag.Perguntar(context.Background(), &a, "quanto tenho?")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Passos) != 0 || res.Chamadas != 1 {
		t.Errorf("modo A chamou ferramenta: %+v", res)
	}
	s := p.sistemas[0]
	if strings.Contains(s, "CÓDIGOS DE ITEM") || strings.Contains(s, "contou") || !strings.Contains(s, "Monitoria de Algoritmos") {
		t.Error("o modo A deve ver as atividades brutas, sem horas calculadas nem catálogo")
	}
}

func TestFerramentaComArgumentoRuimDevolveErroAoModelo(t *testing.T) {
	ag, _, a := preparar(t, ModoB)
	out := ag.executar(&a, llm.Chamada{Nome: "simular_inclusao", Argumentos: json.RawMessage(`{"item":"XX","quantidade":1}`)})
	if !strings.HasPrefix(out, "erro:") {
		t.Errorf("saída: %s", out)
	}
	out = ag.executar(&a, llm.Chamada{Nome: "detalhar_item", Argumentos: json.RawMessage(`"{\"item\":\"IV-I-1\"}"`)})
	if !strings.Contains(out, "item usado 50h de 50h") {
		t.Errorf("detalhar com argumentos em string: %s", out)
	}
}
