package regras_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/fonte"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

func dirDados() string {
	_, arq, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(arq), "..", "..", "dados")
}

func carregar(t *testing.T) (*regras.Regras, fonte.JSON) {
	t.Helper()
	f := fonte.JSON{Dir: dirDados()}
	r, err := f.Regras()
	if err != nil {
		t.Fatal(err)
	}
	return r, f
}

func aluno(t *testing.T, f fonte.JSON, m string) regras.Aluno {
	t.Helper()
	a, err := f.Aluno(m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Totais calculados à mão antes de rodar o código.
func TestSaldoDosAlunosFicticios(t *testing.T) {
	r, f := carregar(t)
	casos := []struct {
		matricula  string
		total      float64
		categorias int
		concluido  bool
	}{
		{"2022101", 165, 3, false},
		{"2023201", 135, 3, false},
		{"2024301", 100, 1, false},
		{"2022401", 200, 1, false},
		{"2023501", 160, 1, false},
		{"2022601", 147.5, 3, false},
		{"2025701", 71, 1, false},
		{"2021801", 220, 3, true},
	}
	for _, c := range casos {
		s := regras.Calcular(r, aluno(t, f, c.matricula), regras.Opcoes{})
		if s.Total != c.total || len(s.CategoriasComHoras) != c.categorias || s.Concluido != c.concluido {
			t.Errorf("%s: total %v, categorias %v, concluído %v; esperado %v, %d, %v\n%s",
				c.matricula, s.Total, s.CategoriasComHoras, s.Concluido, c.total, c.categorias, c.concluido, s.Texto())
		}
	}
}

func TestTetoDaCategoriaCortaMesmoComItemLivre(t *testing.T) {
	r, f := carregar(t)
	// Categoria IV tem 90h; 6 meses de estágio valem 30, mas só cabem 10.
	sim := regras.Simular(r, aluno(t, f, "2022101"),
		regras.Atividade{Item: "IV-II-1", Quantidade: 6, Data: regras.DeveData("2026-12-01")}, regras.Opcoes{})
	if sim.Ganho != 10 {
		t.Fatalf("ganho %v, esperado 10: %+v", sim.Ganho, sim.Nova)
	}
	if !strings.Contains(strings.Join(sim.Nova.Motivos, " "), "categoria IV") {
		t.Errorf("motivo deveria citar a categoria IV: %v", sim.Nova.Motivos)
	}
}

func TestAntesDoIngressoNaoConta(t *testing.T) {
	r, f := carregar(t)
	s := regras.Calcular(r, aluno(t, f, "2024301"), regras.Opcoes{})
	for _, a := range s.Atividades {
		if a.AtividadeID == 301 {
			if a.Contabilizado != 0 || !strings.Contains(a.Motivos[0], "Art. 13") {
				t.Errorf("curso antes do ingresso: %+v", a)
			}
			return
		}
	}
	t.Fatal("atividade 301 não apareceu")
}

func TestAdendoSoValeParaProtocoloDepoisDaVigencia(t *testing.T) {
	r, f := carregar(t)
	a := aluno(t, f, "2023201")
	antes := regras.DeveData("2026-05-20")
	depois := regras.DeveData("2026-06-01")
	nova := regras.Atividade{Item: "III-A-3", Quantidade: 1, Data: regras.DeveData("2026-03-10")}
	nova.Protocolo = &antes
	if g := regras.Simular(r, a, nova, regras.Opcoes{}).Ganho; g != 0 {
		t.Errorf("protocolo antes da vigência rendeu %v", g)
	}
	nova.Protocolo = &depois
	if g := regras.Simular(r, a, nova, regras.Opcoes{}).Ganho; g != 20 {
		t.Errorf("protocolo depois da vigência rendeu %v, esperado 20", g)
	}
}

func TestCoinventorEFaixaDeCargaHoraria(t *testing.T) {
	r, f := carregar(t)
	a := aluno(t, f, "2022601")
	s := regras.Calcular(r, a, regras.Opcoes{})
	for _, x := range s.Atividades {
		if x.AtividadeID == 604 && x.Contabilizado != 12.5 {
			t.Errorf("depósito como coinventor: %+v", x)
		}
	}
	c := aluno(t, f, "2025701")
	g := regras.Simular(r, c, regras.Atividade{Item: "V-II-1", Quantidade: 3, Data: regras.DeveData("2026-12-01")}, regras.Opcoes{})
	if g.Ganho != 0 {
		t.Errorf("curso de 3h rendeu %v", g.Ganho)
	}
}

func TestApenasValidadasIgnoraPendentes(t *testing.T) {
	r, f := carregar(t)
	a := aluno(t, f, "2025701")
	tudo := regras.Calcular(r, a, regras.Opcoes{})
	val := regras.Calcular(r, a, regras.Opcoes{ApenasValidadas: true})
	if tudo.Total != val.Total { // o curso pendente de 3h vale 0 de qualquer forma
		t.Errorf("totais diferentes: %v e %v", tudo.Total, val.Total)
	}
	ok := false
	for _, x := range val.Atividades {
		if x.AtividadeID == 703 && strings.Contains(strings.Join(x.Motivos, " "), "pendente") {
			ok = true
		}
	}
	if !ok {
		t.Error("atividade pendente deveria aparecer com o motivo")
	}
}
