package fonte

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

// GerarSQL transforma os JSON em INSERTs para db/02_dados.sql.
// Os JSON são a fonte; o SQL é gerado (não edite à mão).
func GerarSQL(r *regras.Regras, alunos []regras.Aluno) string {
	var b strings.Builder
	b.WriteString("-- GERADO por `go run ./cmd/horas gerar-seed` a partir de dados/*.json. Não edite à mão.\n\n")
	fmt.Fprintf(&b, "INSERT INTO parametro (nome, valor) VALUES ('total_exigido', %s), ('minimo_categorias', %d);\n\n",
		f(r.TotalExigido), r.MinimoCategorias)
	for i, t := range r.Interpretacoes {
		fmt.Fprintf(&b, "INSERT INTO interpretacao (ordem, texto) VALUES (%d, %s);\n", i+1, q(t))
	}
	b.WriteString("\n")
	for i, c := range r.Categorias {
		fmt.Fprintf(&b, "INSERT INTO categoria (codigo, ordem, nome, teto) VALUES (%s, %d, %s, %s);\n",
			q(c.Codigo), i+1, q(c.Nome), pf(c.Teto))
	}
	b.WriteString("\n")
	for i, s := range r.Subcategorias {
		fmt.Fprintf(&b, "INSERT INTO subcategoria (codigo, ordem, categoria, nome, teto) VALUES (%s, %d, %s, %s, %s);\n",
			q(s.Codigo), i+1, q(s.Categoria), q(s.Nome), pf(s.Teto))
	}
	b.WriteString("\n")
	for i, it := range r.Itens {
		vig := "NULL"
		if it.VigenteDesde != nil {
			vig = q(it.VigenteDesde.String())
		}
		fmt.Fprintf(&b, "INSERT INTO item (codigo, ordem, subcategoria, descricao, unidade, calculo, valor, teto_por_unidade, teto, ch_min, ch_max, ch_acima_de, fator_coinventor, vigente_desde, fonte, documentacao) VALUES (%s, %d, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s);\n",
			q(it.Codigo), i+1, q(it.Subcategoria), q(it.Descricao), q(it.Unidade), q(it.Calculo),
			pf(it.Valor), pf(it.TetoPorUnidade), pf(it.Teto), pf(it.CHMin), pf(it.CHMax), pf(it.CHAcimaDe),
			pf(it.FatorCoinventor), vig, q(it.Fonte), q(it.Documentacao))
	}
	b.WriteString("\n-- Alunos fictícios (nenhum dado real).\n")
	for _, a := range alunos {
		fmt.Fprintf(&b, "INSERT INTO aluno (matricula, nome, ingresso) VALUES (%s, %s, %s);\n", q(a.Matricula), q(a.Nome), q(a.Ingresso.String()))
	}
	b.WriteString("\n")
	for _, a := range alunos {
		for _, at := range a.Atividades {
			prot := "NULL"
			if at.Protocolo != nil {
				prot = q(at.Protocolo.String())
			}
			fmt.Fprintf(&b, "INSERT INTO atividade (id, matricula, item, descricao, quantidade, carga_horaria, coinventor, data_realizacao, data_protocolo, situacao) VALUES (%d, %s, %s, %s, %s, %s, %t, %s, %s, %s);\n",
				at.ID, q(a.Matricula), q(at.Item), q(at.Descricao), f(at.Quantidade), pf(at.CargaHoraria),
				at.Coinventor, q(at.Data.String()), prot, q(at.Situacao))
		}
	}
	return b.String()
}

func q(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func f(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }

func pf(x *float64) string {
	if x == nil {
		return "NULL"
	}
	return f(*x)
}
