package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Anthropic chama a Messages API (POST /v1/messages) direto por HTTP.
type Anthropic struct {
	Chave      string
	NomeModelo string
	URL        string
	Cliente    *http.Client
}

func (a *Anthropic) Nome() string   { return "anthropic" }
func (a *Anthropic) Modelo() string { return a.NomeModelo }

type antBloco struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type antMensagem struct {
	Role    string     `json:"role"`
	Content []antBloco `json:"content"`
}

type antFerramenta struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type antPedido struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []antMensagem   `json:"messages"`
	Tools     []antFerramenta `json:"tools,omitempty"`
}

type antResposta struct {
	Content    []antBloco `json:"content"`
	StopReason string     `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) Conversar(ctx context.Context, sistema string, hist []Mensagem, ferr []Ferramenta) (Resposta, error) {
	p := antPedido{Model: a.NomeModelo, MaxTokens: 2048, System: sistema, Messages: paraAnthropic(hist)}
	for _, f := range ferr {
		p.Tools = append(p.Tools, antFerramenta{Name: f.Nome, Description: f.Descricao, InputSchema: f.Esquema})
	}
	corpo, err := json.Marshal(p)
	if err != nil {
		return Resposta{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.URL+"/v1/messages", bytes.NewReader(corpo))
	if err != nil {
		return Resposta{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", a.Chave)
	req.Header.Set("anthropic-version", "2023-06-01")
	cli := a.Cliente
	if cli == nil {
		cli = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := cli.Do(req)
	if err != nil {
		return Resposta{}, fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	dados, _ := io.ReadAll(resp.Body)
	var r antResposta
	if err := json.Unmarshal(dados, &r); err != nil {
		return Resposta{}, fmt.Errorf("anthropic: resposta HTTP %d ilegível: %s", resp.StatusCode, limitar(string(dados)))
	}
	if resp.StatusCode != http.StatusOK || r.Error != nil {
		msg := string(dados)
		if r.Error != nil {
			msg = r.Error.Type + ": " + r.Error.Message
		}
		return Resposta{}, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, limitar(msg))
	}
	out := Resposta{Uso: Uso{Entrada: r.Usage.InputTokens, Saida: r.Usage.OutputTokens}}
	for _, b := range r.Content {
		switch b.Type {
		case "text":
			out.Texto += b.Text
		case "tool_use":
			out.Chamadas = append(out.Chamadas, Chamada{ID: b.ID, Nome: b.Name, Argumentos: b.Input})
		}
	}
	return out, nil
}

// paraAnthropic converte o histórico; resultados de ferramenta seguidos viram
// uma única mensagem de usuário com vários tool_result.
func paraAnthropic(hist []Mensagem) []antMensagem {
	var out []antMensagem
	for _, m := range hist {
		switch m.Papel {
		case PapelUsuario:
			out = append(out, antMensagem{Role: "user", Content: []antBloco{{Type: "text", Text: m.Texto}}})
		case PapelAssistente:
			var bl []antBloco
			if m.Texto != "" {
				bl = append(bl, antBloco{Type: "text", Text: m.Texto})
			}
			for _, c := range m.Chamadas {
				in := c.Argumentos
				if len(in) == 0 {
					in = json.RawMessage(`{}`)
				}
				bl = append(bl, antBloco{Type: "tool_use", ID: c.ID, Name: c.Nome, Input: in})
			}
			out = append(out, antMensagem{Role: "assistant", Content: bl})
		case PapelFerramenta:
			b := antBloco{Type: "tool_result", ToolUseID: m.ChamadaID, Content: m.Texto}
			if n := len(out); n > 0 && out[n-1].Role == "user" && len(out[n-1].Content) > 0 && out[n-1].Content[0].Type == "tool_result" {
				out[n-1].Content = append(out[n-1].Content, b)
			} else {
				out = append(out, antMensagem{Role: "user", Content: []antBloco{b}})
			}
		}
	}
	return out
}

func limitar(s string) string {
	if len(s) > 500 {
		return s[:500] + "…"
	}
	return s
}
