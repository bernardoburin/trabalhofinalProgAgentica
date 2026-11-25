package avaliacao_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/avaliacao"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/fonte"
)

// As respostas esperadas das 30 perguntas foram escritas à mão a partir do
// regulamento; este teste confere os números delas contra o motor de regras.
func TestGabaritoBateComOMotor(t *testing.T) {
	_, arq, _, _ := runtime.Caller(0)
	raiz := filepath.Join(filepath.Dir(arq), "..", "..")
	f := fonte.JSON{Dir: filepath.Join(raiz, "dados")}
	r, err := f.Regras()
	if err != nil {
		t.Fatal(err)
	}
	ps, err := avaliacao.CarregarPerguntas(filepath.Join(raiz, "testes", "perguntas.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 30 {
		t.Errorf("%d perguntas, esperado 30", len(ps))
	}
	for _, e := range avaliacao.ConferirGabarito(r, f, ps) {
		t.Error(e)
	}
}
