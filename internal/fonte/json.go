// Package fonte carrega regras e alunos, dos arquivos JSON ou do Postgres.
package fonte

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

// Fonte entrega as regras e os alunos, venham de onde vierem.
type Fonte interface {
	Regras() (*regras.Regras, error)
	Aluno(matricula string) (regras.Aluno, error)
	Alunos() ([]regras.Aluno, error)
}

// JSON lê dados/regras.json e dados/alunos.json.
type JSON struct{ Dir string }

func (j JSON) Regras() (*regras.Regras, error) {
	var r regras.Regras
	if err := lerJSON(filepath.Join(j.Dir, "regras.json"), &r); err != nil {
		return nil, err
	}
	if err := r.Preparar(); err != nil {
		return nil, err
	}
	return &r, nil
}

func (j JSON) Alunos() ([]regras.Aluno, error) {
	var as []regras.Aluno
	err := lerJSON(filepath.Join(j.Dir, "alunos.json"), &as)
	return as, err
}

func (j JSON) Aluno(m string) (regras.Aluno, error) {
	as, err := j.Alunos()
	if err != nil {
		return regras.Aluno{}, err
	}
	for _, a := range as {
		if a.Matricula == m {
			return a, nil
		}
	}
	return regras.Aluno{}, fmt.Errorf("aluno %s não encontrado", m)
}

func lerJSON(caminho string, v any) error {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", caminho, err)
	}
	return nil
}
