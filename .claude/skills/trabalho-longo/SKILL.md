---
name: trabalho-longo
description: Monta o esquema para um trabalho grande continuar sozinho quando o limite de uso acaba no meio (inclusive de madrugada). São três partes — o plano em arquivo com as regras e o andamento, o agendamento "continua o plano" a cada ~30 min e o fechamento. Usar quando o pedido for grande demais para uma vez só (vários PRs, muitas horas), quando pedirem "deixa rodando à noite" ou "continua depois que o limite acabar", e para SUGERIR o esquema antes de começar um pedido assim.
---

# Trabalho longo (continuar depois do limite de uso)

Um pedido com vários PRs ou muitas horas pode esbarrar no limite de uso da sessão no meio do caminho. Quando o limite volta, ninguém está olhando para retomar.

O esquema:
- um **plano em arquivo**, que diz o que foi feito e o que falta;
- um **agendamento na sessão**, que manda "continua o plano" de tempos em tempos;
- o **fechamento**, que desliga tudo.

Qualquer retomada, inclusive depois de o contexto ser resumido, lê o plano e segue do primeiro item aberto.

## 1. Quando sugerir

Antes de começar, se o pedido tiver vários PRs, várias issues ou cara de muitas horas, **ofereça** o esquema em uma frase. Exemplo: "Isso dá umas N horas. Quer que eu deixe agendado para continuar sozinho se o limite de uso acabar no meio?" Não monte sem a pessoa concordar.

Diga junto as limitações (seção 3): a sessão precisa ficar aberta e o computador ligado.

## 2. O plano em arquivo

Crie `ferramentas/claude/PLANO-<assunto>.md`. A pasta `ferramentas/` é ignorada pelo git: o plano é de quem pediu e pode ter dados pessoais (valores, nomes de contas), então **não vai para o repositório**. Modelo:

```markdown
# Plano: <assunto> (pedido de AAAA-MM-DD)

**Para retomar:** quando chegar "continua o plano <assunto>", leia este arquivo inteiro e continue do primeiro item que
não está [x]. Atualize este arquivo ao terminar cada etapa (marque [x] e anote o PR).

**Agendamento:** job <id do CronCreate> ("<cron>"). Vive só nesta sessão e expira em 7 dias. Cancelar com CronDelete
quando tudo estiver [x].

## Pedido (resumo fiel)
<o que a pessoa pediu, com as palavras dela quando importar>

## Regras de trabalho (não pular)
- <as do CLAUDE.md que mais pesam aqui: branch pela issue, Conventional Commits, versão no Directory.Build.props,
  Manual e CHANGELOG no mesmo PR, ADR quando couber, um PR por vez quando mexem nos mesmos arquivos>
- <as que a pessoa pediu: ex. "espere meu ok antes de cada merge", "não compile no meu PC enquanto eu jogo">
- <o que espera a pessoa: UAC, publicação, decisões que contrariam uma ADR Aceita>

## Ordem e andamento
1. [ ] **#N <título>**: <o que falta>. (Ao terminar: "PR #M, no develop".)
2. [ ] ...
N. [ ] **Fechamento:** cancelar o agendamento e avisar (seção 5).

## Achados (só local; NÃO publicar)
<números e nomes que não podem ir para issue/PR, porque o repositório é público>
```

Se a sessão tiver memória persistente, grave uma memória curta apontando para o arquivo ("plano em `ferramentas/claude/PLANO-<assunto>.md`; 'continua o plano' retoma"). Assim uma sessão nova também acha o plano.

## 3. O agendamento

Na sessão do Claude Code, carregue as ferramentas de agendamento (`ToolSearch` com `select:CronCreate,CronDelete,CronList`) e crie **um** job recorrente:

- **Quando:** `13,43 * * * *` (duas vezes por hora, fora do :00 e do :30, para não cair junto com outros agendamentos).
- **Prompt:** `continua o plano <assunto>: leia <caminho do plano> e continue do primeiro item não marcado com [x], seguindo as regras de lá (responda em português). Se já estiver trabalhando numa etapa, só siga. Quando todos os itens estiverem [x], cancele este agendamento com CronDelete e avise.`
- Anote o id do job no plano.

**Limitações, para dizer à pessoa:**
- O job é **da sessão**. Ele só dispara com o Claude Code aberto nessa conversa, o computador ligado e sem hibernar, e a sessão ociosa (não interrompe um trabalho em andamento).
- Expira em **7 dias**.
- Ele não fura o limite de uso: o disparo que cair antes de o limite voltar não faz nada, e o primeiro depois disso retoma.
- Fechou a sessão? O plano continua no arquivo. Numa sessão nova, "continua o plano <assunto>" retoma, e dá para recriar o agendamento.

## 4. Durante o trabalho

- **Atualize o plano a cada etapa:** marque [x], anote o PR e escreva o que falta na etapa em andamento. É a única memória que sobrevive ao resumo do contexto.
- **Pedido novo no meio:** a pessoa mandou outra coisa ou mudou uma regra? Acrescente no plano na hora (em "Regras" ou como item novo), com a data.
- **O que espera a pessoa** (UAC, publicação no computador dela, ok de merge, decisão que contraria uma ADR Aceita): marque "espera o usuário" e siga para o próximo item que não depende dela.
- **Não faça sozinho** o que é difícil de desfazer: force push, apagar branch, apagar dados.
- **Mostre progresso** em mensagens curtas entre as etapas: a pessoa pode estar acompanhando.

## 5. Fechamento

Quando todos os itens estiverem [x]:
1. Cancele o agendamento (`CronDelete <id>`; `CronList` mostra os que existem).
2. Avise a pessoa com o resumo:
   - o que entrou, com os números dos PRs;
   - a versão, de qual para qual;
   - o que ficou de fora e por quê;
   - o que ela precisa fazer (ex.: publicar, conferir algo no app do banco).
3. Marque no plano que terminou. Se gravou memória, atualize-a ("plano concluído em AAAA-MM-DD") ou apague.
