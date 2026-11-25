package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ollama chama POST /api/chat de um Ollama local. Custo zero; precisa de um
// modelo com suporte a ferramentas (qwen3, llama3.1 e outros).
type Ollama struct {
	URL        string
	NomeModelo string
	Cliente    *http.Client
}

func (o *Ollama) Nome() string   { return "ollama" }
func (o *Ollama) Modelo() string { return o.NomeModelo }

type olChamada struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type olMensagem struct {
	Role      string      `json:"role"`
	Content   string      `json:"content"`
	ToolCalls []olChamada `json:"tool_calls,omitempty"`
	ToolName  string      `json:"tool_name,omitempty"`
}

type olFerramenta struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type olPedido struct {
	Model    string         `json:"model"`
	Messages []olMensagem   `json:"messages"`
	Tools    []olFerramenta `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type olResposta struct {
	Message         olMensagem `json:"message"`
	PromptEvalCount int        `json:"prompt_eval_count"`
	EvalCount       int        `json:"eval_count"`
	Error           string     `json:"error"`
}

func (o *Ollama) Conversar(ctx context.Context, sistema string, hist []Mensagem, ferr []Ferramenta) (Resposta, error) {
	p := olPedido{
		Model:  o.NomeModelo,
		Stream: false,
		// O regulamento inteiro tem ~8 mil tokens; o contexto padrão do Ollama
		// é menor e cortaria o documento sem avisar.
		Options: map[string]any{"num_ctx": 32768, "temperature": 0},
	}
	p.Messages = append(p.Messages, olMensagem{Role: "system", Content: sistema})
	for _, m := range hist {
		switch m.Papel {
		case PapelUsuario:
			p.Messages = append(p.Messages, olMensagem{Role: "user", Content: m.Texto})
		case PapelAssistente:
			om := olMensagem{Role: "assistant", Content: m.Texto}
			for _, c := range m.Chamadas {
				var oc olChamada
				oc.Function.Name = c.Nome
				oc.Function.Arguments = c.Argumentos
				om.ToolCalls = append(om.ToolCalls, oc)
			}
			p.Messages = append(p.Messages, om)
		case PapelFerramenta:
			p.Messages = append(p.Messages, olMensagem{Role: "tool", Content: m.Texto, ToolName: m.Nome})
		}
	}
	for _, f := range ferr {
		var of olFerramenta
		of.Type = "function"
		of.Function.Name = f.Nome
		of.Function.Description = f.Descricao
		of.Function.Parameters = f.Esquema
		p.Tools = append(p.Tools, of)
	}
	corpo, err := json.Marshal(p)
	if err != nil {
		return Resposta{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.URL+"/api/chat", bytes.NewReader(corpo))
	if err != nil {
		return Resposta{}, err
	}
	req.Header.Set("content-type", "application/json")
	cli := o.Cliente
	if cli == nil {
		cli = &http.Client{Timeout: 10 * time.Minute} // modelo local em CPU é lento
	}
	resp, err := cli.Do(req)
	if err != nil {
		return Resposta{}, fmt.Errorf("ollama em %s: %w (o Ollama está rodando? ollama serve)", o.URL, err)
	}
	defer resp.Body.Close()
	dados, _ := io.ReadAll(resp.Body)
	var r olResposta
	if err := json.Unmarshal(dados, &r); err != nil {
		return Resposta{}, fmt.Errorf("ollama: resposta HTTP %d ilegível: %s", resp.StatusCode, limitar(string(dados)))
	}
	if resp.StatusCode != http.StatusOK || r.Error != "" {
		return Resposta{}, fmt.Errorf("ollama: HTTP %d: %s (o modelo %s foi baixado? ollama pull %s)", resp.StatusCode, limitar(r.Error), o.NomeModelo, o.NomeModelo)
	}
	out := Resposta{Texto: semPensamento(r.Message.Content), Uso: Uso{Entrada: r.PromptEvalCount, Saida: r.EvalCount}}
	for i, c := range r.Message.ToolCalls {
		out.Chamadas = append(out.Chamadas, Chamada{ID: fmt.Sprintf("ollama-%d", i), Nome: c.Function.Name, Argumentos: c.Function.Arguments})
	}
	return out, nil
}

// semPensamento tira o bloco <think>…</think> que alguns modelos (qwen3)
// devolvem dentro do texto em versões antigas do Ollama.
func semPensamento(s string) string {
	ini := strings.Index(s, "<think>")
	fim := strings.Index(s, "</think>")
	if ini >= 0 && fim > ini {
		s = s[:ini] + s[fim+len("</think>"):]
	}
	return strings.TrimSpace(s)
}
