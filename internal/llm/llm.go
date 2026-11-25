// Package llm fala com o modelo: Claude pela API da Anthropic ou um modelo
// local pelo Ollama. Só usa a biblioteca padrão.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// Papel de uma mensagem no histórico.
const (
	PapelUsuario    = "usuario"
	PapelAssistente = "assistente"
	PapelFerramenta = "ferramenta" // resultado de uma chamada de ferramenta
)

type Chamada struct {
	ID         string          `json:"id"`
	Nome       string          `json:"nome"`
	Argumentos json.RawMessage `json:"argumentos"`
}

type Mensagem struct {
	Papel     string    `json:"papel"`
	Texto     string    `json:"texto,omitempty"`
	Chamadas  []Chamada `json:"chamadas,omitempty"`   // assistente pedindo ferramentas
	ChamadaID string    `json:"chamada_id,omitempty"` // resposta de ferramenta
	Nome      string    `json:"nome,omitempty"`       // nome da ferramenta respondida
}

type Ferramenta struct {
	Nome      string         `json:"nome"`
	Descricao string         `json:"descricao"`
	Esquema   map[string]any `json:"esquema"` // JSON Schema dos argumentos
}

type Uso struct {
	Entrada int `json:"tokens_entrada"`
	Saida   int `json:"tokens_saida"`
}

type Resposta struct {
	Texto    string
	Chamadas []Chamada
	Uso      Uso
}

// Provedor é um modelo que conversa e pode pedir ferramentas.
type Provedor interface {
	Nome() string
	Modelo() string
	Conversar(ctx context.Context, sistema string, historico []Mensagem, ferramentas []Ferramenta) (Resposta, error)
}

// DoAmbiente escolhe o provedor por HORAS_LLM (ollama ou anthropic).
func DoAmbiente() (Provedor, error) {
	modelo := os.Getenv("HORAS_MODELO")
	switch p := envOu("HORAS_LLM", "ollama"); p {
	case "ollama":
		return &Ollama{URL: envOu("OLLAMA_URL", "http://localhost:11434"), NomeModelo: ou(modelo, "qwen3:8b")}, nil
	case "anthropic":
		chave := os.Getenv("ANTHROPIC_API_KEY")
		if chave == "" {
			return nil, fmt.Errorf("HORAS_LLM=anthropic precisa de ANTHROPIC_API_KEY")
		}
		return &Anthropic{Chave: chave, NomeModelo: ou(modelo, "claude-sonnet-5-5"), URL: envOu("ANTHROPIC_URL", "https://api.anthropic.com")}, nil
	default:
		return nil, fmt.Errorf("HORAS_LLM=%q desconhecido; use ollama ou anthropic", p)
	}
}

func envOu(nome, padrao string) string { return ou(os.Getenv(nome), padrao) }

func ou(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
