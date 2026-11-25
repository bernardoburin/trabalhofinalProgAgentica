# Proposta: assistente de horas complementares

Programação Agêntica · IFSul Passo Fundo · Trabalho final
Grupo: <nomes> · Data da proposta: 25/11/2026

## Pergunta

Um LLM com o regulamento de atividades complementares consegue dizer a um aluno se uma atividade nova ainda vai contar (limites que ele não viu) e qual é a melhor forma de usar as horas que faltam? Ou ele precisa de ferramentas que calculem o saldo?

## Hipótese (25/11/2026)

Com só o regulamento no contexto, o modelo vai acertar as regras simples ("monitoria vale 25h por semestre") e errar quando o limite é composto. O caso típico: o item ainda cabe, mas a subcategoria, a categoria (IV e V têm teto de 100) ou o total de 200h já estourou. Nas recomendações de "melhor aproveitamento", ele vai sugerir a atividade que rende mais horas por unidade sem descontar o que o aluno já usou naquele teto. Motivo: o regulamento tem limites em três níveis, mais regras espalhadas (adendo de 05/2026, mínimo de 2 categorias, Art. 12º e Art. 13º), e somar e cortar tetos em vários passos é onde modelos de linguagem costumam errar. Quando o modelo tiver uma ferramenta que calcula o saldo e simula a inclusão, a maior parte desses erros deve sumir, e o que sobrar vai estar nas ambiguidades do próprio regulamento.

## Cenário

**Pessoa:** um aluno do 7º semestre de Ciência da Computação. **Pedido, nas palavras dele:** "Tenho umas 140 horas e não sei se o que tô pensando em fazer vai contar. Quero saber o que fazer pra fechar as 200 sem perder tempo com coisa que não vale."

## O que vamos construir

Um programa de linha de comando em Go que conversa com o Claude por API. Os dados ficam num Postgres local (Docker).

- **Regras:** o Anexo A e o adendo viram tabelas (categoria, subcategoria e item, com teto e forma de cálculo).
- **Alunos:** 10 a 15 alunos fictícios, com atividades em situações escolhidas para exercitar os limites.
- **Ferramentas do agente:** `saldo(aluno)` e `simular_inclusao(aluno, atividade)`. São código determinístico, com usuário do banco só de leitura.
- **Três versões para comparar:**
  - (A) só o regulamento no prompt;
  - (B) regulamento mais as ferramentas;
  - (C) o gabarito, calculado só por código.

O regulamento inteiro cabe no contexto (cerca de 12 páginas), então vai inteiro no prompt e o RAG fica fora do escopo. Se faltar tempo, cortamos a versão B antes de cortar o gabarito.

## Como vamos observar

- **Conjunto de teste:** cerca de 30 perguntas, metade de limite oculto e metade de melhor aproveitamento. As respostas esperadas saem do gabarito e são conferidas à mão contra o regulamento por alguém que não escreveu o código.
- **Medida:** acerto de A e B em cada tipo de pergunta, com o tipo de cada erro anotado.
- **Registro:** fica na pasta `registro/` do repositório, com commit a cada etapa. Antes de cada etapa, anotamos a previsão de tempo e o que será delegado ao agente de código. Ao longo do trabalho, registramos:
  - horas trabalhadas e gasto com a API;
  - o que o agente escreveu e o que reescrevemos;
  - onde travamos;
  - modelo e versões usados.

## Cuidados

- Nenhum dado real de aluno: tudo fictício.
- A chave da API fica no `.env`, fora do git.
- Teto de gasto com a API: <R$ X>, com o gasto registrado.
- O adendo não tem texto selecionável, então será transcrito à mão; essa transcrição também vai para o registro.
