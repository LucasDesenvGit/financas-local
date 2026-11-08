---
name: reportar-erro
description: Recebe um erro copiado pelo botão "Reportar para o Claude" (Configurações › Erros do Finanças Local, #332) — o texto começa com /reportar-erro — e cuida dele — procura uma issue aberta com o mesmo erro, analisa onde está no código e a causa provável, pergunta se resolve agora, só cataloga ou se é do ambiente, e segue. Usar quando a mensagem começar com /reportar-erro, ou quando pedirem para "reportar", "catalogar" ou "abrir issue" de um erro de Configurações › Erros.
---

# Reportar um erro do site (Finanças Local)

O botão **"Reportar para o Claude"**, em Configurações › Erros, copia o erro num texto que começa com `/reportar-erro`. A pessoa cola o texto aqui. O site não fala com o Claude direto: isso pediria uma saída de rede nova e uma ADR (#332).

O texto traz:
- a mensagem, a origem, o nível e a tela;
- quantas vezes aconteceu, a primeira e a última vez;
- quem registrou e a pilha, quando há;
- a versão do sistema.

Antes de gravar, o site já trocou tokens, senhas, CPF e valores em reais por `[removido]`.

## 0. Antes de publicar qualquer coisa: o repositório é público

A limpeza do site é por padrão de texto e não pega tudo. **Releia o texto antes de pôr na issue**, e troque por `[removido]` o que sobrou:
- nome de pessoa;
- nome de conta, de cartão ou de investimento escolhido pela pessoa;
- descrição de transação;
- e-mail, telefone, endereço;
- valor sem o "R$";
- caminho com o nome do usuário do computador (`C:\Users\<nome>\...`).

Na dúvida, não publique o trecho: descreva-o ("a descrição de uma transação").

## 1. Já existe issue?

Procure pelas palavras mais específicas da mensagem, sem números nem `[removido]`:

```bash
gh issue list --state open --search "<trecho da mensagem> in:title,body" --limit 10
```

Achou a mesma? Comente nela as ocorrências novas (quantas vezes, a última, a versão). Não abra outra. Diga à pessoa qual é e pule para o passo 3, com a issue que já existe.

## 2. Analisar

Antes de perguntar, entenda o erro:
- **Onde no código:**
  - a origem diz a área: "API", "Open Finance", "Investimentos", "Câmbio", "Cripto", "Sistema" e "Front-end";
  - o "Registrado por" diz o pacote ou a tela;
  - a pilha diz o arquivo.

  Procure a mensagem com `Grep` em `src/backend` (Go) ou em `src/frontend/src/app` (Angular).
- **A causa provável,** em uma ou duas frases, e o que ainda não dá para saber sem reproduzir.
- **É do ambiente de desenvolvimento?** Alguns erros não são do site publicado:
  - `Failed to fetch dynamically imported module` em `localhost:4200`: o servidor de desenvolvimento recompilou (troca de branch, por exemplo) com a aba aberta, e o pedaço antigo do JavaScript sumiu. Recarregar a página resolve;
  - erros com `localhost:4200` em geral, ou da porta de desenvolvimento da API.

  No site publicado, a tela é servida pela própria API, em `https://127.0.0.1:5443`.
- **Gravidade:**
  - perde ou mostra dado errado → alta;
  - quebra uma tela ou uma ação → média;
  - aviso que se resolve sozinho (internet fora, banco fora do ar) → baixa.

## 3. Perguntar o que fazer

Use o `AskUserQuestion` com três opções, a recomendada primeiro e marcada como "(Recomendado)":
- **Resolver agora:** recomende quando a causa está clara e a correção é pequena;
- **Só catalogar:** quando é grande, incerta ou de baixa gravidade;
- **É do ambiente, não precisa de issue:** quando o passo 2 mostrou que é do desenvolvimento.

Diga numa frase, na pergunta, a causa provável e onde ela está, para a pessoa decidir sem ler a análise toda.

## 4. Seguir a escolha

- **Do ambiente:** explique o que fazer (ex.: recarregar a página) e sugira marcar o erro como resolvido em Configurações › Erros. Não abra issue.
- **Só catalogar:**
  1. Abra a issue com `gh issue create --assignee @me`, no modelo das outras:
     - 🎯 Objetivo;
     - 😣 Hoje: o erro, já relido no passo 0, e a análise;
     - 📋 O que fazer;
     - ✅ Critérios de aceite.
  2. Ponha as etiquetas: `bug`, a da área (`api`, `web`, `open-finance`, `investimentos`, `scripts`) e a da prioridade (`prioridade: alta|média|baixa`).
  3. Ligue a issue como sub-issue ao épico do assunto (`gh issue list --label "épico: ..."`). Veja o comando no `CLAUDE.md`, em "Git / commits".
- **Resolver agora:**
  1. Abra a issue como em "Só catalogar".
  2. Siga o fluxo do `CLAUDE.md`:
     - `gh issue edit <N> --add-label "em progresso"`;
     - a branch `fix/<assunto>` a partir do `develop`;
     - a correção;
     - o **teste de regressão** que falha antes da correção (Go: ADR 0051; front-end: `testes/jest/regressao/`);
     - a versão e o CHANGELOG;
     - o PR com `Closes #N`.
  3. No fim, peça à pessoa para marcar o erro como resolvido em Configurações › Erros quando a versão com a correção estiver publicada. Se ele voltar depois disso, ele reaparece em aberto, numa linha nova.

## 5. Resumo

Diga em poucas linhas:
- o que era;
- a issue (nova ou a que já existia);
- o que foi feito;
- o que a pessoa precisa fazer (recarregar, publicar, marcar como resolvido).
