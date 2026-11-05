# 0058. Pastas do front-end: páginas por grupo do menu, `compartilhado/` dividido em `ui/` e por assunto

- **Status:** Aceita
- **Data:** 2026-11-05
- **Relacionadas:** [0046](0046-menu-lateral-em-grupos-que-abrem-e-fecham.md) (os grupos do menu); [0056](0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md) (um pacote por assunto no Go); [0013](0013-testes-do-front-end-com-jest.md) (o teste de arquitetura, #476)

## Contexto

Em 04/11/2026, `src/app/paginas/` tinha 37 arquivos e cerca de 15 mil linhas numa pasta só, com componentes e lógica pura misturados (`limite-cartao.ts`, `metas-plano.ts`, `recorrentes.ts`, `painel-avisos.ts`…). `src/app/compartilhado/` tinha 40 arquivos, e só parte deles era compartilhada de verdade: cinco eram usados só pelo `shell` (a barra de cima e o menu), e dois só por uma tela. Cada tela nova caía numa pasta cada vez maior, e o componente "compartilhado" que só uma tela usa escondia de quem ele é (#634).

O back-end em Go já tem um pacote por assunto (ADR 0056), e o menu do site já agrupa as telas por assunto (ADR 0046).

## Alternativas consideradas

- **Deixar como estava** — nada a mover; contra: a pasta só cresce, e o PR que um dia organizar fica maior e conflita com mais branches abertas.
- **Uma pasta por tela** (`paginas/metas/`, `paginas/transacoes/`…) — o mais comum no style guide do Angular; contra: são 17 telas, a maioria com um arquivo só, e as abas e janelas de uma tela (Investimentos › Câmbio, Contas a pagar › Assinaturas) ficariam soltas ou em pastas de um arquivo.
- **Pastas pelo tipo de arquivo** (`componentes/`, `funcoes/`) — contra: separa a tela da lógica dela, o contrário do style guide ("organize by feature areas").
- **Páginas pelos grupos do menu** (escolhida) — o mesmo assunto que a pessoa vê no menu; poucas pastas, de 2 a 9 arquivos cada.

## Decisão

`paginas/` tem uma subpasta por grupo do menu (`MENU` em `layout/shell.component.ts`), e `compartilhado/` se divide em `ui/` (sem assunto do domínio) e por assunto. O que só uma parte usa mora com ela.

```
src/app/
  paginas/
    painel/        painel; rosca-categorias (só o Painel usa)
    movimentacao/  transações, contas e cartões, contas a pagar e as abas delas (assinaturas e recorrentes,
                   renomear, limite do cartão), conferir banco
    planejamento/  orçamentos (e a sugestão), metas (plano e evolução)
    patrimonio/    investimentos e as abas (câmbio, cripto, FIIs, Open Finance, conferir, reserva), empréstimos
    analises/      relatórios, visão anual, projeção
    ajustes/       categorias, configurações e as abas (parametrização, aparência, erros); auditoria
    manual/        manual e manual-conteudo
    entrar.component.ts   (fora do menu)
  compartilhado/
    ui/            sem assunto do domínio: modal, confirmação, avisos rápidos, paginação, lista suspensa, dica,
                   menu de ações, seletores (múltiplo, mês, tema), campos de data e de valor, ícones
    graficos/      gráfico, gráfico explicado, linhas, misto, cores, "mais análises"
    bancos/        bancos, serviços, logo do banco, selo do serviço
    lancamentos/   tipos de lançamento, ícone da categoria, faturas no painel, KPI das faturas abertas
    glossario/     glossário e <app-termo>
    moeda.pipe.ts, utilidades.ts   (usados por quase tudo)
  layout/          o shell e o que só ele usa: botão de privacidade, calculadora, menu do usuário, notificações
                   (e os avisos do sino: painel-avisos), status da sincronização
  core/            serviços e modelos, inclusive o AvisosService (os avisos rápidos que qualquer parte abre)
```

- **Página:** na pasta do grupo do menu em que ela aparece. Aba, janela ou função pura de uma tela ficam na pasta da tela, mesmo que o nome não seja do grupo (`reserva-conferencia.ts` é de Investimentos, então `patrimonio/`).
- **`compartilhado/`:** só o que duas ou mais partes usam. Sem assunto do domínio (`ui/`), ou com assunto (`graficos/`, `bancos/`, `lancamentos/`, `glossario/`). Na raiz, só o que quase tudo usa.
- **Só o `shell` usa:** vai para `layout/`. **Só uma tela usa:** vai para a pasta da tela.
- **O `compartilhado/ui/` não importa `bancos/`, `lancamentos/` nem `glossario/`** (regra 6 do teste de arquitetura): o que não sabe do domínio recebe o dado pronto.

*Julgamento próprio (#634):* a issue punha `recorrentes.ts` e `assinaturas-recorrentes` em `planejamento/` e `reserva-conferencia.ts` em `movimentacao/`. Pelo grafo de imports, os dois primeiros são usados só por Contas e Contas a pagar (Movimentação), e o terceiro só por Investimentos (Patrimônio); ficaram com quem os usa. `confirmacao.component`/`.service` (ADR 0057, que entrou depois da issue) ficaram juntos em `ui/`, porque o serviço é usado por oito telas. E a issue levava o cálculo dos avisos do sino (`painel-avisos.ts`) para o `core`: desde a #327, ele e o `painel-avisos.component` só servem ao sino das notificações, então foram para `layout/`, pela regra de cima, e o `core` continua só com serviços e modelos. O `AvisosService` foi para o `core`, e o componente que mostra os avisos rápidos, para o `ui/`.

## Consequências

- **Positivas:** a pasta diz de quem é o arquivo; tela nova cai numa pasta de poucos arquivos; o que está em `compartilhado/` é compartilhado de verdade.
- **Negativas:**
  - os imports ficam um nível mais fundos (`../../core/...`);
  - mudar uma tela de grupo no menu pede mover o arquivo junto, ou a pasta deixa de bater com o menu;
  - um PR aberto antes da mudança, que cria arquivo nas pastas antigas, entra sem conflito, mas no lugar antigo. Depois de cada merge, confira que não voltou arquivo para a raiz de `paginas/` ou `compartilhado/` (fora `entrar`, `moeda.pipe` e `utilidades`).
- **Na prática:**
  - tela nova vai na pasta do grupo do menu dela; grupo novo no menu ganha pasta nova;
  - componente compartilhado novo: `ui/` se não sabe nada do domínio, senão a pasta do assunto (ou uma nova, com o assunto no nome);
  - o teste de arquitetura (`npm run test:arquitetura`) acha as páginas com rota em `paginas/<grupo>/x.component` e confere quem importa quem pela primeira pasta (`core`, `compartilhado`, `layout`, `paginas`).
