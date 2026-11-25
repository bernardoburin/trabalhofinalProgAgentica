// Comando horas: assistente de horas complementares (trabalho final de
// Programação Agêntica). Rode `go run ./cmd/horas ajuda`.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/agente"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/avaliacao"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/fonte"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/llm"
	"github.com/bernardoburin/trabalhofinalProgAgentica/internal/regras"
)

const ajuda = `Uso: horas <comando> [opções]

Comandos:
  saldo <matrícula> [-validadas]        saldo calculado pelo motor (gabarito)
  simular <matrícula> <item> <qtd>      quanto uma atividade nova renderia
        [-ch N] [-coinventor] [-data AAAA-MM-DD] [-protocolo AAAA-MM-DD]
  perguntar [-modo A|B] [-aluno M] "pergunta"
                                        pergunta ao agente (modo A: só documento;
                                        modo B: documento + ferramentas)
  avaliar [-modo A|B] [-so 1,5,7]       roda as perguntas de testes/perguntas.json
                                        e grava resultados/*.jsonl e *.md
  gabarito                              confere as respostas esperadas com o motor
  gerar-seed                            gera db/02_dados.sql a partir de dados/*.json
  itens                                 lista os códigos de item

Ambiente:
  HORAS_FONTE   banco (padrão) ou json
  DATABASE_URL  padrão postgres://horas_leitura:leitura@localhost:5432/horas
  HORAS_LLM     ollama (padrão, local e grátis) ou anthropic
  HORAS_MODELO  padrão qwen3:8b (ollama) ou claude-sonnet-5-5 (anthropic)
  ANTHROPIC_API_KEY, OLLAMA_URL
  HORAS_HOJE    data de "hoje" para o agente (padrão: a data do sistema)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(ajuda)
		os.Exit(2)
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	var err error
	args := os.Args[2:]
	switch os.Args[1] {
	case "saldo":
		err = cmdSaldo(args)
	case "simular":
		err = cmdSimular(args)
	case "perguntar":
		err = cmdPerguntar(ctx, args)
	case "avaliar":
		err = cmdAvaliar(ctx, args)
	case "gabarito":
		err = cmdGabarito()
	case "gerar-seed":
		err = cmdGerarSeed()
	case "itens":
		err = cmdItens()
	case "ajuda", "-h", "--help":
		fmt.Print(ajuda)
	default:
		err = fmt.Errorf("comando desconhecido %q; rode: horas ajuda", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func env(nome, padrao string) string {
	if v := os.Getenv(nome); v != "" {
		return v
	}
	return padrao
}

func dirDados() string { return env("HORAS_DADOS", "dados") }

func abrirFonte() (fonte.Fonte, *regras.Regras, error) {
	var f fonte.Fonte
	switch env("HORAS_FONTE", "banco") {
	case "banco":
		f = fonte.Banco{DSN: env("DATABASE_URL", "postgres://horas_leitura:leitura@localhost:5432/horas")}
	case "json":
		f = fonte.JSON{Dir: dirDados()}
	default:
		return nil, nil, errors.New("HORAS_FONTE deve ser banco ou json")
	}
	r, err := f.Regras()
	return f, r, err
}

func hoje() (regras.Data, error) {
	if v := os.Getenv("HORAS_HOJE"); v != "" {
		return regras.NovaData(v)
	}
	return regras.NovaData(time.Now().Format("2006-01-02"))
}

func cmdSaldo(args []string) error {
	fs := flag.NewFlagSet("saldo", flag.ExitOnError)
	val := fs.Bool("validadas", false, "conta só atividades validadas")
	js := fs.Bool("json", false, "saída em JSON")
	fs.Parse(reordenar(args))
	if fs.NArg() != 1 {
		return errors.New("uso: horas saldo <matrícula>")
	}
	f, r, err := abrirFonte()
	if err != nil {
		return err
	}
	a, err := f.Aluno(fs.Arg(0))
	if err != nil {
		return err
	}
	s := regras.Calcular(r, a, regras.Opcoes{ApenasValidadas: *val})
	if *js {
		return imprimirJSON(s)
	}
	fmt.Print(s.Texto())
	return nil
}

func cmdSimular(args []string) error {
	fs := flag.NewFlagSet("simular", flag.ExitOnError)
	ch := fs.Float64("ch", 0, "carga horária do curso (itens com mínimo)")
	coin := fs.Bool("coinventor", false, "aluno é coinventor")
	data := fs.String("data", "", "data de realização (padrão: hoje)")
	prot := fs.String("protocolo", "", "data do protocolo (padrão: hoje)")
	fs.Parse(reordenar(args))
	if fs.NArg() != 3 {
		return errors.New("uso: horas simular <matrícula> <item> <quantidade>")
	}
	q, err := strconv.ParseFloat(strings.Replace(fs.Arg(2), ",", ".", 1), 64)
	if err != nil {
		return fmt.Errorf("quantidade inválida: %s", fs.Arg(2))
	}
	f, r, err := abrirFonte()
	if err != nil {
		return err
	}
	a, err := f.Aluno(fs.Arg(0))
	if err != nil {
		return err
	}
	if r.Item(fs.Arg(1)) == nil {
		return fmt.Errorf("item %s não existe; rode: horas itens", fs.Arg(1))
	}
	h, err := hoje()
	if err != nil {
		return err
	}
	nova := regras.Atividade{Item: fs.Arg(1), Descricao: "simulada", Quantidade: q, Coinventor: *coin, Data: h}
	p := h
	if *data != "" {
		if nova.Data, err = regras.NovaData(*data); err != nil {
			return err
		}
	}
	if *prot != "" {
		if p, err = regras.NovaData(*prot); err != nil {
			return err
		}
	}
	nova.Protocolo = &p
	if *ch > 0 {
		nova.CargaHoraria = ch
	}
	return imprimirJSON(regras.Simular(r, a, nova, regras.Opcoes{}))
}

func novoAgente(modo string, r *regras.Regras) (*agente.Agente, error) {
	modo = strings.ToUpper(modo)
	if modo != agente.ModoA && modo != agente.ModoB {
		return nil, errors.New("modo deve ser A ou B")
	}
	p, err := llm.DoAmbiente()
	if err != nil {
		return nil, err
	}
	h, err := hoje()
	if err != nil {
		return nil, err
	}
	return &agente.Agente{Provedor: p, Regras: r, Modo: modo, Hoje: h}, nil
}

func cmdPerguntar(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("perguntar", flag.ExitOnError)
	modo := fs.String("modo", "B", "A (só documento) ou B (documento + ferramentas)")
	mat := fs.String("aluno", "", "matrícula do aluno (vazio: pergunta geral)")
	verb := fs.Bool("v", false, "mostra as chamadas de ferramenta")
	fs.Parse(reordenar(args))
	pergunta := strings.Join(fs.Args(), " ")
	if pergunta == "" {
		return errors.New(`uso: horas perguntar -aluno 2022101 "quanto falta?"`)
	}
	f, r, err := abrirFonte()
	if err != nil {
		return err
	}
	ag, err := novoAgente(*modo, r)
	if err != nil {
		return err
	}
	var al *regras.Aluno
	if *mat != "" {
		a, err := f.Aluno(*mat)
		if err != nil {
			return err
		}
		al = &a
	}
	res, err := ag.Perguntar(ctx, al, pergunta)
	if *verb {
		for _, p := range res.Passos {
			fmt.Fprintf(os.Stderr, "→ %s %s\n%s\n", p.Ferramenta, p.Argumentos, p.Resultado)
		}
	}
	if err != nil {
		return err
	}
	fmt.Println(res.Resposta)
	fmt.Fprintf(os.Stderr, "\n[%s %s, modo %s, %d chamadas ao modelo, %d+%d tokens, %s]\n",
		res.Provedor, res.Modelo, res.Modo, res.Chamadas, res.Uso.Entrada, res.Uso.Saida, res.Duracao.Round(time.Millisecond))
	return nil
}

func cmdAvaliar(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("avaliar", flag.ExitOnError)
	modo := fs.String("modo", "B", "A ou B")
	arq := fs.String("perguntas", filepath.Join("testes", "perguntas.json"), "arquivo de perguntas")
	so := fs.String("so", "", "só estas perguntas (ex.: 1,5,7)")
	saida := fs.String("saida", "resultados", "pasta dos resultados")
	fs.Parse(reordenar(args))
	ps, err := avaliacao.CarregarPerguntas(*arq)
	if err != nil {
		return err
	}
	if *so != "" {
		quer := map[string]bool{}
		for _, x := range strings.Split(*so, ",") {
			quer[strings.TrimSpace(x)] = true
		}
		var filtro []avaliacao.Pergunta
		for _, p := range ps {
			if quer[strconv.Itoa(p.ID)] {
				filtro = append(filtro, p)
			}
		}
		ps = filtro
	}
	f, r, err := abrirFonte()
	if err != nil {
		return err
	}
	ag, err := novoAgente(*modo, r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*saida, 0o755); err != nil {
		return err
	}
	modelo := strings.NewReplacer(":", "-", "/", "-").Replace(ag.Provedor.Modelo())
	base := filepath.Join(*saida, fmt.Sprintf("%s_modo-%s_%s_%s", time.Now().Format("2006-01-02_1504"), ag.Modo, ag.Provedor.Nome(), modelo))
	jl, err := os.Create(base + ".jsonl")
	if err != nil {
		return err
	}
	defer jl.Close()
	md, err := os.Create(base + ".md")
	if err != nil {
		return err
	}
	defer md.Close()
	w := bufio.NewWriter(md)
	defer w.Flush()
	fmt.Fprintf(w, "# Avaliação: modo %s, %s %s, %s\n\nHoje para o agente: %s. Marque cada resposta: certo, parcial ou errado. Quem marca não deve ter escrito o código.\n\n",
		ag.Modo, ag.Provedor.Nome(), ag.Provedor.Modelo(), time.Now().Format("02/01/2006 15:04"), ag.Hoje)

	var tokE, tokS, falhas int
	var dur time.Duration
	err = avaliacao.Rodar(ctx, ag, f, ps, func(reg avaliacao.Registro) {
		b, _ := json.Marshal(reg)
		jl.Write(append(b, '\n'))
		tokE += reg.Resultado.Uso.Entrada
		tokS += reg.Resultado.Uso.Saida
		dur += reg.Resultado.Duracao
		fmt.Fprintf(w, "## Pergunta %d (%s, aluno %s)\n\n**Pergunta:** %s\n\n**Esperado:** %s\n\n**Resposta:**\n\n%s\n\n",
			reg.Pergunta, reg.Tipo, ou(reg.Aluno, "—"), reg.Texto, reg.RespostaEsperada, citar(reg.Resultado.Resposta))
		if reg.Erro != "" {
			falhas++
			fmt.Fprintf(w, "**Erro:** %s\n\n", reg.Erro)
		}
		if len(reg.Resultado.Passos) > 0 {
			var nomes []string
			for _, p := range reg.Resultado.Passos {
				nomes = append(nomes, fmt.Sprintf("%s %s", p.Ferramenta, p.Argumentos))
			}
			fmt.Fprintf(w, "**Ferramentas:** %s\n\n", strings.Join(nomes, " · "))
		}
		fmt.Fprintf(w, "**Avaliação:** [ ] certo  [ ] parcial  [ ] errado · tipo do erro: \n\n")
		fmt.Fprintf(os.Stderr, "pergunta %d: %d chamadas, %s\n", reg.Pergunta, reg.Resultado.Chamadas, reg.Resultado.Duracao.Round(time.Millisecond))
	})
	fmt.Fprintf(w, "---\n\nTotal: %d perguntas, %d erros de execução, %d tokens de entrada, %d de saída, %s.\n", len(ps), falhas, tokE, tokS, dur.Round(time.Second))
	fmt.Fprintf(os.Stderr, "Resultados em %s.jsonl e %s.md (%d tokens de entrada, %d de saída)\n", base, base, tokE, tokS)
	return err
}

func cmdGabarito() error {
	f, r, err := abrirFonte()
	if err != nil {
		return err
	}
	ps, err := avaliacao.CarregarPerguntas(filepath.Join("testes", "perguntas.json"))
	if err != nil {
		return err
	}
	erros := avaliacao.ConferirGabarito(r, f, ps)
	for _, e := range erros {
		fmt.Println("DIVERGE:", e)
	}
	if len(erros) > 0 {
		return fmt.Errorf("%d divergências entre o gabarito e o motor", len(erros))
	}
	fmt.Printf("Gabarito confere com o motor (%d perguntas, fonte %s).\n", len(ps), env("HORAS_FONTE", "banco"))
	return nil
}

func cmdGerarSeed() error {
	f := fonte.JSON{Dir: dirDados()}
	r, err := f.Regras()
	if err != nil {
		return err
	}
	as, err := f.Alunos()
	if err != nil {
		return err
	}
	dest := filepath.Join("db", "02_dados.sql")
	if err := os.WriteFile(dest, []byte(fonte.GerarSQL(r, as)), 0o644); err != nil {
		return err
	}
	fmt.Printf("Gerei %s (%d itens, %d alunos). Para recarregar o banco: docker compose down -v && docker compose up -d\n", dest, len(r.Itens), len(as))
	return nil
}

func cmdItens() error {
	_, r, err := abrirFonte()
	if err != nil {
		return err
	}
	for _, it := range r.Itens {
		fmt.Printf("%-9s %s (%s)\n", it.Codigo, it.Descricao, it.Unidade)
	}
	return nil
}

func imprimirJSON(v any) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}

func citar(s string) string {
	if s == "" {
		return "> (vazia)"
	}
	return "> " + strings.ReplaceAll(s, "\n", "\n> ")
}

func ou(s, p string) string {
	if s == "" {
		return p
	}
	return s
}

// reordenar põe as opções antes dos argumentos, para aceitar
// `horas saldo 2022101 -validadas` além de `horas saldo -validadas 2022101`.
func reordenar(args []string) []string {
	var opc, pos []string
	comValor := map[string]bool{"-ch": true, "-data": true, "-protocolo": true, "-modo": true, "-aluno": true,
		"-perguntas": true, "-so": true, "-saida": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			opc = append(opc, a)
			nome := strings.SplitN(strings.TrimLeft(a, "-"), "=", 2)[0]
			if comValor["-"+nome] && !strings.Contains(a, "=") && i+1 < len(args) {
				i++
				opc = append(opc, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(opc, pos...)
}
