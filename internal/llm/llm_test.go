package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicMontaPedidoELeChamada(t *testing.T) {
	var pedido map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("pedido errado: %s %v", r.URL.Path, r.Header)
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &pedido)
		io.WriteString(w, `{"content":[{"type":"text","text":"vou ver"},{"type":"tool_use","id":"tu1","name":"consultar_saldo","input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":3}}`)
	}))
	defer srv.Close()
	a := &Anthropic{Chave: "k", NomeModelo: "m", URL: srv.URL}
	hist := []Mensagem{
		{Papel: PapelUsuario, Texto: "oi"},
		{Papel: PapelAssistente, Chamadas: []Chamada{{ID: "a", Nome: "x"}, {ID: "b", Nome: "y"}}},
		{Papel: PapelFerramenta, ChamadaID: "a", Texto: "ra"},
		{Papel: PapelFerramenta, ChamadaID: "b", Texto: "rb"},
	}
	r, err := a.Conversar(context.Background(), "sis", hist, []Ferramenta{{Nome: "consultar_saldo", Esquema: map[string]any{"type": "object"}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Texto != "vou ver" || len(r.Chamadas) != 1 || r.Chamadas[0].ID != "tu1" || r.Uso.Entrada != 10 {
		t.Errorf("resposta: %+v", r)
	}
	msgs := pedido["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("os dois tool_result deveriam ir numa só mensagem: %d mensagens", len(msgs))
	}
	ultima := msgs[2].(map[string]any)["content"].([]any)
	if len(ultima) != 2 || ultima[0].(map[string]any)["type"] != "tool_result" {
		t.Errorf("tool_results: %v", ultima)
	}
	if pedido["system"] != "sis" {
		t.Errorf("system: %v", pedido["system"])
	}
}

func TestAnthropicErroHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	}))
	defer srv.Close()
	_, err := (&Anthropic{Chave: "k", NomeModelo: "m", URL: srv.URL}).Conversar(context.Background(), "", []Mensagem{{Papel: PapelUsuario, Texto: "oi"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "authentication_error") {
		t.Errorf("erro: %v", err)
	}
}

func TestOllamaMontaPedidoELeChamada(t *testing.T) {
	var pedido olPedido
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &pedido)
		io.WriteString(w, `{"message":{"role":"assistant","content":"<think>hmm</think>","tool_calls":[{"function":{"name":"simular_inclusao","arguments":{"item":"IV-II-1","quantidade":6}}}]},"prompt_eval_count":5,"eval_count":2}`)
	}))
	defer srv.Close()
	o := &Ollama{URL: srv.URL, NomeModelo: "qwen3:8b"}
	hist := []Mensagem{{Papel: PapelUsuario, Texto: "oi"}, {Papel: PapelFerramenta, Nome: "x", Texto: "r"}}
	r, err := o.Conversar(context.Background(), "sis", hist, []Ferramenta{{Nome: "simular_inclusao"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Texto != "" || len(r.Chamadas) != 1 || !strings.Contains(string(r.Chamadas[0].Argumentos), "IV-II-1") {
		t.Errorf("resposta: %+v", r)
	}
	if pedido.Messages[0].Role != "system" || pedido.Messages[2].Role != "tool" || pedido.Options["num_ctx"] == nil {
		t.Errorf("pedido: %+v", pedido)
	}
}
