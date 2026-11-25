-- Esquema do assistente de horas complementares.
-- Roda automaticamente na primeira subida do contêiner (docker-entrypoint-initdb.d).

CREATE TABLE parametro (
    nome  text PRIMARY KEY,
    valor numeric NOT NULL
);

CREATE TABLE interpretacao (
    ordem int PRIMARY KEY,
    texto text NOT NULL
);

CREATE TABLE categoria (
    codigo text PRIMARY KEY,
    ordem  int  NOT NULL,
    nome   text NOT NULL,
    teto   numeric            -- NULL = sem teto de categoria
);

CREATE TABLE subcategoria (
    codigo    text PRIMARY KEY,
    ordem     int  NOT NULL,
    categoria text NOT NULL REFERENCES categoria(codigo),
    nome      text NOT NULL,
    teto      numeric
);

CREATE TABLE item (
    codigo           text PRIMARY KEY,
    ordem            int  NOT NULL,
    subcategoria     text NOT NULL REFERENCES subcategoria(codigo),
    descricao        text NOT NULL,
    unidade          text NOT NULL DEFAULT '',
    calculo          text NOT NULL CHECK (calculo IN ('horas', 'fracao', 'fixo')),
    valor            numeric,          -- horas por unidade (fixo) ou fator (fracao)
    teto_por_unidade numeric,          -- "com limite de N" por projeto, evento ou disciplina
    teto             numeric,          -- coluna "Limite Máximo de Horas" do Anexo A
    ch_min           numeric,          -- carga horária mínima exigida
    ch_max           numeric,          -- carga horária máxima da faixa
    ch_acima_de      numeric,          -- carga horária estritamente acima de
    fator_coinventor numeric,          -- 0,5 onde o anexo prevê 50% para coinventores
    vigente_desde    date,             -- itens do adendo: data mínima do protocolo
    fonte            text NOT NULL DEFAULT '',
    documentacao     text NOT NULL DEFAULT ''
);

CREATE TABLE aluno (
    matricula text PRIMARY KEY,
    nome      text NOT NULL,
    ingresso  date NOT NULL
);

CREATE TABLE atividade (
    id              int PRIMARY KEY,
    matricula       text NOT NULL REFERENCES aluno(matricula),
    item            text NOT NULL REFERENCES item(codigo),
    descricao       text NOT NULL,
    quantidade      numeric NOT NULL,  -- horas, dias, meses, semestres, unidades ou CH do curso, conforme o item
    carga_horaria   numeric,           -- CH do curso, quando o item exige mínimo (idiomas)
    coinventor      boolean NOT NULL DEFAULT false,
    data_realizacao date NOT NULL,
    data_protocolo  date,
    situacao        text NOT NULL CHECK (situacao IN ('validada', 'pendente', 'indeferida'))
);

CREATE INDEX atividade_matricula ON atividade (matricula);

-- Usuário do agente: só leitura. Nem um prompt injection consegue alterar horas.
CREATE ROLE horas_leitura LOGIN PASSWORD 'leitura';
GRANT CONNECT ON DATABASE horas TO horas_leitura;
GRANT USAGE ON SCHEMA public TO horas_leitura;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO horas_leitura;
