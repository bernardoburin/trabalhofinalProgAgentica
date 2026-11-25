# Assistente de horas complementares

Trabalho final de Programação Agêntica (IFSul Passo Fundo). A pergunta e a hipótese estão em `proposta.md`.

O programa responde a dúvidas de um aluno sobre as horas complementares do BCC (quanto ainda conta, que limite ele não viu, como aproveitar melhor as horas que faltam). Há três jeitos de responder, para comparar:

- **Modo A:** o modelo recebe o regulamento inteiro e as atividades do aluno, e faz tudo sozinho.
- **Modo B:** o mesmo, mais ferramentas que calculam saldo e simulam inclusões com as regras oficiais.
- **Gabarito:** o motor de regras em Go, sem modelo nenhum (`internal/regras`).

## Pré-requisitos

- Go 1.25 ou mais novo
- Docker (para o Postgres)
- Um modelo:
  - **Ollama** (padrão, local e grátis): instale em <https://ollama.com> e rode `ollama pull qwen3:8b`. Um modelo de 8B precisa de cerca de 8 GB de RAM livres e é lento em CPU.
  - **Claude pela API** (pago, à parte da assinatura do Claude): uma chave da Anthropic.

## Subir

```sh
docker compose up -d          # Postgres em localhost:5432, já com tabelas e dados fictícios
go mod tidy                   # baixa o pgx e gera o go.sum
go test ./...                 # motor de regras, gabarito, clientes do modelo e agente
go run ./cmd/horas gabarito   # confere as 30 respostas esperadas lendo do Postgres
```

O último comando é o teste de que o banco subiu certo: ele carrega regras e alunos do Postgres e confere os números do gabarito. Sem o banco, use `HORAS_FONTE=json`.

## Usar

```sh
go run ./cmd/horas saldo 2022101
go run ./cmd/horas simular 2022101 IV-II-1 6          # 6 meses de estágio: rende só 10h
go run ./cmd/horas itens                               # códigos de item
go run ./cmd/horas perguntar -modo B -aluno 2022101 -v "Consegui um estágio de 6 meses. Quanto ele me dá?"
go run ./cmd/horas avaliar -modo A                     # as 30 perguntas, modo A
go run ./cmd/horas avaliar -modo B -so 1,6,20          # só algumas, modo B
```

O `avaliar` grava em `resultados/` um `.jsonl` (respostas, ferramentas chamadas, tokens, tempo) e um `.md` para correção à mão. Quem corrige não deve ter escrito o código. Commite os resultados: eles são o registro do experimento.

### Claude pela API

```sh
export HORAS_LLM=anthropic
export ANTHROPIC_API_KEY=sk-ant-...      # nunca no git
export HORAS_MODELO=claude-sonnet-5-5    # opcional
```

**Atenção:** se `ANTHROPIC_API_KEY` estiver exportada no terminal onde você abre o Claude Code, o Claude Code passa a cobrar da API em vez de usar a sua assinatura. Exporte a chave só no terminal onde roda o `horas`, ou use `set -a; source .env; set +a` nesse terminal (o Go não lê `.env` sozinho).

## Variáveis

| Variável | Padrão | Para quê |
|---|---|---|
| `HORAS_FONTE` | `banco` | `banco` (Postgres) ou `json` (arquivos em `dados/`) |
| `DATABASE_URL` | `postgres://horas_leitura:leitura@localhost:5432/horas` | usuário só de leitura |
| `HORAS_LLM` | `ollama` | `ollama` ou `anthropic` |
| `HORAS_MODELO` | `qwen3:8b` / `claude-sonnet-5-5` | modelo |
| `OLLAMA_URL` | `http://localhost:11434` | |
| `HORAS_HOJE` | data do sistema | fixe (ex.: `2026-11-25`) para repetir a avaliação nas mesmas condições |

## Pastas

```
dados/regras.json        Anexo A e adendo transcritos (fonte única das regras) e as interpretações adotadas
dados/alunos.json        8 alunos fictícios, montados para exercitar os limites
db/01_schema.sql         tabelas e usuário só de leitura
db/02_dados.sql          GERADO de dados/*.json: go run ./cmd/horas gerar-seed
regulamento/             regulamento e adendo em texto, que vão no prompt
testes/perguntas.json    30 perguntas, com armadilha, resposta esperada e verificação numérica
internal/regras          motor de cálculo (gabarito)
internal/fonte           leitura do JSON e do Postgres
internal/llm             clientes HTTP do Claude e do Ollama
internal/agente          prompt, ferramentas e laço do agente (modos A e B)
internal/avaliacao       roda as perguntas e confere o gabarito
cmd/horas                linha de comando
```

Para mudar regras ou alunos, edite `dados/*.json`, rode `go run ./cmd/horas gerar-seed` e recrie o banco com `docker compose down -v && docker compose up -d`.

## Decisões que valem para o artigo

- **As contas são código, não modelo.** Os limites estão em três níveis (item, subcategoria e categoria), mais o total de 200h e o mínimo de 2 categorias. O motor corta cada atividade pelo que resta em cada nível, em ordem de data.
- **Sem RAG.** O regulamento inteiro tem cerca de 8 mil tokens e vai todo no prompt.
- **O adendo não tem texto no PDF** (é imagem) e foi transcrito à mão.
- **Interpretações.** Onde o regulamento é ambíguo, a escolha feita está em `dados/regras.json`, no campo `interpretacoes`. Cinco das 30 perguntas testam justamente essas ambiguidades.
- **O agente só lê o banco.** O usuário `horas_leitura` só tem `SELECT`.

## Como foi feito

O código e as perguntas foram gerados com ajuda de IA (Claude). Antes da semana 2, registre isso no `registro/`, junto com o que você revisou, o que mudou e o que ainda não entende.
