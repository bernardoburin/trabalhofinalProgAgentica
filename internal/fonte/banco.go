package fonte

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

// Banco lê regras e alunos do Postgres. Use o usuário só de leitura
// (horas_leitura): o agente nunca precisa escrever.
type Banco struct{ DSN string }

func (b Banco) conectar(ctx context.Context) (*pgx.Conn, error) {
	c, err := pgx.Connect(ctx, b.DSN)
	if err != nil {
		return nil, fmt.Errorf("conectar ao Postgres (%s): %w; o banco está de pé? (docker compose up -d)", semSenha(b.DSN), err)
	}
	return c, nil
}

func (b Banco) Regras() (*regras.Regras, error) {
	ctx := context.Background()
	c, err := b.conectar(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close(ctx)

	r := &regras.Regras{Fonte: "Postgres"}
	rows, err := c.Query(ctx, `SELECT nome, valor::float8 FROM parametro`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var nome string
		var valor float64
		if err := rows.Scan(&nome, &valor); err != nil {
			rows.Close()
			return nil, err
		}
		switch nome {
		case "total_exigido":
			r.TotalExigido = valor
		case "minimo_categorias":
			r.MinimoCategorias = int(valor)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = c.Query(ctx, `SELECT texto FROM interpretacao ORDER BY ordem`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return nil, err
		}
		r.Interpretacoes = append(r.Interpretacoes, t)
	}
	rows.Close()

	rows, err = c.Query(ctx, `SELECT codigo, nome, teto::float8 FROM categoria ORDER BY ordem`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x regras.Categoria
		if err := rows.Scan(&x.Codigo, &x.Nome, &x.Teto); err != nil {
			rows.Close()
			return nil, err
		}
		r.Categorias = append(r.Categorias, x)
	}
	rows.Close()

	rows, err = c.Query(ctx, `SELECT codigo, categoria, nome, teto::float8 FROM subcategoria ORDER BY ordem`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x regras.Subcategoria
		if err := rows.Scan(&x.Codigo, &x.Categoria, &x.Nome, &x.Teto); err != nil {
			rows.Close()
			return nil, err
		}
		r.Subcategorias = append(r.Subcategorias, x)
	}
	rows.Close()

	rows, err = c.Query(ctx, `SELECT codigo, subcategoria, descricao, unidade, calculo,
		valor::float8, teto_por_unidade::float8, teto::float8, ch_min::float8, ch_max::float8,
		ch_acima_de::float8, fator_coinventor::float8, vigente_desde, fonte, documentacao
		FROM item ORDER BY ordem`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x regras.Item
		var vig *time.Time
		if err := rows.Scan(&x.Codigo, &x.Subcategoria, &x.Descricao, &x.Unidade, &x.Calculo,
			&x.Valor, &x.TetoPorUnidade, &x.Teto, &x.CHMin, &x.CHMax, &x.CHAcimaDe, &x.FatorCoinventor,
			&vig, &x.Fonte, &x.Documentacao); err != nil {
			rows.Close()
			return nil, err
		}
		if vig != nil {
			x.VigenteDesde = &regras.Data{Time: *vig}
		}
		r.Itens = append(r.Itens, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.Preparar(); err != nil {
		return nil, err
	}
	return r, nil
}

func (b Banco) Alunos() ([]regras.Aluno, error) {
	return b.consultarAlunos("")
}

func (b Banco) Aluno(m string) (regras.Aluno, error) {
	as, err := b.consultarAlunos(m)
	if err != nil {
		return regras.Aluno{}, err
	}
	if len(as) == 0 {
		return regras.Aluno{}, fmt.Errorf("aluno %s não encontrado no banco", m)
	}
	return as[0], nil
}

func (b Banco) consultarAlunos(matricula string) ([]regras.Aluno, error) {
	ctx := context.Background()
	c, err := b.conectar(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close(ctx)

	rows, err := c.Query(ctx, `SELECT matricula, nome, ingresso FROM aluno
		WHERE $1 = '' OR matricula = $1 ORDER BY matricula`, matricula)
	if err != nil {
		return nil, err
	}
	var alunos []regras.Aluno
	indice := map[string]int{}
	for rows.Next() {
		var a regras.Aluno
		var ing time.Time
		if err := rows.Scan(&a.Matricula, &a.Nome, &ing); err != nil {
			rows.Close()
			return nil, err
		}
		a.Ingresso = regras.Data{Time: ing}
		indice[a.Matricula] = len(alunos)
		alunos = append(alunos, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = c.Query(ctx, `SELECT id, matricula, item, descricao, quantidade::float8,
		carga_horaria::float8, coinventor, data_realizacao, data_protocolo, situacao
		FROM atividade WHERE $1 = '' OR matricula = $1 ORDER BY data_realizacao, id`, matricula)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var at regras.Atividade
		var m string
		var data time.Time
		var prot *time.Time
		if err := rows.Scan(&at.ID, &m, &at.Item, &at.Descricao, &at.Quantidade, &at.CargaHoraria,
			&at.Coinventor, &data, &prot, &at.Situacao); err != nil {
			return nil, err
		}
		at.Data = regras.Data{Time: data}
		if prot != nil {
			at.Protocolo = &regras.Data{Time: *prot}
		}
		if i, ok := indice[m]; ok {
			alunos[i].Atividades = append(alunos[i].Atividades, at)
		}
	}
	return alunos, rows.Err()
}

func semSenha(dsn string) string {
	// Mostra o DSN sem a senha nas mensagens de erro.
	for i := 0; i+3 < len(dsn); i++ {
		if dsn[i:i+3] == "://" {
			rest := dsn[i+3:]
			for j := 0; j < len(rest); j++ {
				if rest[j] == '@' {
					user := rest[:j]
					for k := 0; k < len(user); k++ {
						if user[k] == ':' {
							return dsn[:i+3] + user[:k] + ":***" + rest[j:]
						}
					}
					return dsn
				}
			}
		}
	}
	return dsn
}
