---
name: angular-no-projeto
description: Como se escreve Angular no Finanças Local (src/frontend) — tela nova, mudança em tela, componente, gráfico, filtro, formulário, CSS e revisão de PR com front-end, com as regras do projeto e a fonte de cada uma. Usar ao pedirem "tela nova", "mexe na tela", "componente", "gráfico", "filtro", "CSS", "UX", "acessibilidade", "avalia a tela", "revisa o front-end", e ao pegar issue com a etiqueta web ou que mexe em src/frontend. Depois de pronto, a skill testar-no-navegador confere no navegador.
model: sonnet
---

# Angular no Finanças Local

Você escreve, avalia e revisa o front-end do projeto (`src/frontend`). O que vale **sempre** está aqui; conferir a tela pronta no navegador é a skill `testar-no-navegador`.

**Cada regra mora num lugar só.** Esta skill diz o caminho e aponta a fonte; não copie a regra para cá. Achou uma regra aqui que diverge da fonte, vale a fonte: corrija a skill no mesmo PR.

| Assunto | Fonte (a regra está lá) |
|---|---|
| UI/UX: as referências obrigatórias (Shneiderman, Nielsen, WCAG 2.2, NN/g, WAI-ARIA) e os padrões já adotados | `CLAUDE.md`, "UI/UX — referências obrigatórias" (já está no seu contexto) |
| `Eager`, `ApiService`, standalone, signals, pastas de `src/app` | `CLAUDE.md`, "Arquitetura › Front-end", e o teste de arquitetura (`testes-arquitetura/arquitetura.spec.ts`) |
| Mesma origem, sem CORS, nada de terceiros | [ADR 0004](../../../docs/adr/0004-front-end-publicado-no-wwwroot-da-api.md) e [ADR 0034](../../../docs/adr/0034-sem-sri-no-build-do-front-end.md) (CSP sem script inline) |
| Preferência de exibição no `localStorage` (`financas.*`) | [ADR 0017](../../../docs/adr/0017-preferencias-de-exibicao-no-navegador.md) |
| Estado na URL, detalhe por navegação, chips de filtro | [ADR 0018](../../../docs/adr/0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) |
| Glossário e `<app-termo>` | [ADR 0020](../../../docs/adr/0020-glossario-unico-com-balao-na-top-layer.md) |
| Gráficos, `<app-mais-analises>`, telas densas | [ADR 0021](../../../docs/adr/0021-componentes-padrao-para-graficos-e-telas-densas.md) |
| Manual do site como dados | [ADR 0026](../../../docs/adr/0026-manual-do-site-como-dados-no-front-end.md) e `CLAUDE.md`, "Manual do site sempre junto" |
| Interruptor, caixa de marcar, campo de valor, campo de data, lista suspensa, seleção em tabela | [ADR 0032](../../../docs/adr/0032-controles-padrao-interruptor-e-campo-de-valor.md) |
| Menu lateral em grupos | [ADR 0046](../../../docs/adr/0046-menu-lateral-em-grupos-que-abrem-e-fecham.md) |
| Testes: Jest × navegador, tempo, importação pesada | [ADR 0013](../../../docs/adr/0013-testes-do-front-end-com-jest.md), [ADR 0041](../../../docs/adr/0041-testes-no-navegador-com-playwright.md), [ADR 0047](../../../docs/adr/0047-ci-paralela-so-testes-afetados-no-pr.md) e `CONTRIBUINDO.md`, "Testes rápidos" |
| Conferir no navegador, prints, pegadinhas do Playwright | skill `testar-no-navegador` |

## Referências (leia quando o passo pedir)

| Arquivo | Quando ler |
|---|---|
| [`referencias/estilo.md`](referencias/estilo.md) | antes de escrever código, e sempre que o angular.dev, um guia da comunidade ou um revisor parecer dizer outra coisa |
| [`referencias/componentes.md`](referencias/componentes.md) | antes de fazer gráfico, campo, filtro, interruptor, seletor, balão ou modal: o componente já existe, use e não refaça |
| [`referencias/testes.md`](referencias/testes.md) | antes de escrever teste: o que vai no Jest, o que vai no navegador, e o que rodar antes do push |
| [`referencias/revisao.md`](referencias/revisao.md) | ao avaliar uma tela ou revisar um PR com front-end, e no fim do seu próprio trabalho |
| [`referencias/exemplos.md`](referencias/exemplos.md) | antes de criar tela, função pura, teste ou preferência: copie a forma de uma tela que já existe |

## Tela nova

1. **Issue e branch** como em todo PR (`CLAUDE.md`, "Git / commits").
2. **ADR antes do código** quando a tela trouxer dependência nova, padrão de UI novo ou desvio de um padrão adotado (`CLAUDE.md`, "Decisões de arquitetura").
3. **Endpoint:** funcionalidade nova fora do núcleo é feita em Go, atrás do .NET (`CLAUDE.md`; skill `go-no-projeto`). No front-end: o modelo em `core/modelos.ts` e o método no `ApiService` (`core/api.service.ts`), nunca `HttpClient` no componente.
4. **Lógica fora do componente:** cálculo, texto e decisão vão para funções puras num `.ts` ao lado da tela (`paginas/<tela>.ts`) ou em `compartilhado/` se mais de uma tela usa, testadas no Jest sem `TestBed` (`referencias/testes.md`). A divisão das páginas grandes ainda está em discussão na #475: siga o que ela decidir quando a ADR for aceita.
5. **Componente** em `paginas/<tela>.component.ts`, na forma de `referencias/estilo.md` (standalone, `Eager`, template inline, `inject()`, signals, `@if`/`@for`), com os componentes de `compartilhado/` (`referencias/componentes.md`).
6. **Rota e menu:** `loadComponent` e `title: '<Tela> · Finanças Local'` em `app.routes.ts`; a tela no grupo do assunto em `MENU` (`layout/shell.component.ts`, ADR 0046).
7. **UI/UX:** cada decisão de desenho cita o princípio da tabela do `CLAUDE.md` que a sustenta, separando o que vem da fonte do que é julgamento seu.
8. **Manual:** seção da tela em `paginas/manual-conteudo.ts` e a rota em `AJUDA_POR_ROTA` (ADR 0026); termo técnico novo no glossário (`compartilhado/glossario.ts`) e, se for do domínio, em `docs/arquitetura/dominio.md`.
9. **Testes:** a tela em `TELAS` (`testes-navegador/base.ts`) e os endpoints dela em `testes-navegador/api-falsa.ts`; a lógica no Jest (`referencias/testes.md`).

## Mudança em tela que já existe

- Leia a tela inteira e os testes dela (`grep -rl <nome> testes testes-navegador`) antes de mexer; siga a forma dela, mesmo que outra tela faça diferente.
- Não aproveite para refatorar a tela: o que achar fora do pedido, liste.
- Bug corrigido ganha teste que falha antes da correção, em `testes/regressao/` (ADR 0013).
- Mudou o que o usuário vê: o Manual no mesmo PR, ou "Manual: não se aplica" na descrição.

## Antes de dizer que terminou

Em `src/frontend`:

```bash
npm run test:tipos                        # a CI roda; o npm test sozinho não confere tipos
NAVEGADOR=chromium npm run test:afetados  # Jest + build + navegador só no que mudou, com o Chromium da CI (#286)
npm run test:arquitetura                  # mexeu em import ou criou arquivo: só local (#476), a CI não roda
```

- Depois, a skill `testar-no-navegador`: os prints das telas mudadas, olhados de verdade, e o teste do que é novo.
- **Node:** se o do sistema não for o do projeto, `export PATH="$(pwd)/../../ferramentas/node:$PATH"`. Não rodou: diga isso e não afirme que passou.
- **O que a CI já pega** (tipos, Jest, tempo dos testes, telas sem erro de console nem de CSP, aviso do Manual, versão, CHANGELOG, CodeQL): rode antes, mas não gaste texto do PR explicando. O que a CI **não** pega é onde está o valor da revisão: [`referencias/revisao.md`](referencias/revisao.md).

## Como entregar

Separe o que conferiu no código ou no navegador (com `arquivo:linha` ou o print) do que é julgamento seu, cite o princípio de UI/UX de cada decisão de desenho, diga o que rodou e o resultado (inclusive o que falhou ou não rodou), e o que ficou de fora, com o motivo. PR: o checklist do `CONTRIBUINDO.md` e do modelo de PR; merge só com o "pode" de quem pediu, para aquele PR.
