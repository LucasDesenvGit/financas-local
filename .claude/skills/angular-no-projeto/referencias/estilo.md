# Estilo do Angular no projeto

Como o código do front-end é escrito aqui, com a fonte de cada ponto. As contagens são do `develop` de 01/11/2026 (`src/frontend/src/app`, 85 arquivos `.ts`); servem para mostrar que o ponto é a prática, não para ser mantidas em dia.

## O que seguimos

| Ponto | Como fica aqui | Fonte |
|---|---|---|
| Detecção de mudança | `changeDetection: ChangeDetectionStrategy.Eager` em **todo** componente novo. Sem ele, o padrão do Angular 22 é OnPush, e a tela que guarda estado em campo comum **não atualiza, sem erro nenhum** | `CLAUDE.md`, "Arquitetura"; o app roda com zone.js (`provideZoneChangeDetection` em `app.config.ts`) |
| Exceção: OnPush | só quando **todo** o estado do componente é signal (ou `input()`); hoje só `paginas/patrimonio/rendimento-investimentos-of.component.ts` | *Inferido* (#559): a regra não está escrita em ADR; na dúvida, `Eager` |
| Componente | standalone (sem `standalone: true`, que já é o padrão), `imports: [...]` só com o que o template usa | `CONTRIBUINDO.md`, "Convenções de código" |
| Template e CSS | template inline (`template: \`...\``, 0 `templateUrl`); CSS no `src/styles.css` global, e `styles: [...]` no componente só para o que é dele (35 arquivos) | prática do código; a ADR 0059 leva para `templateUrl`/`styleUrl` o template de mais de 150 linhas, depois do pré-requisito dela (os testes que leem o template como texto) |
| Estado | `signal`, `computed` e `effect` para o que é novo; campo comum continua valendo nas telas que já o usam (por isso o `Eager`) | `CONTRIBUINDO.md`; [Signals](https://angular.dev/guide/signals) |
| Entrada e saída | `input()`, `input.required()` e `output()` (25 e 13 arquivos); `@Input`/`@Output` só no `grafico.component.ts`, que é antigo | [Accepting data with input properties](https://angular.dev/guide/components/inputs) |
| Injeção | `inject(ApiService)` no campo (50 arquivos), nunca no construtor | [Style guide: prefer the inject function](https://angular.dev/style-guide#prefer-the-inject-function-over-constructor-parameter-injection) |
| Fluxo no template | `@if`, `@for` (com `track`), `@switch`; nada de `*ngIf`/`*ngFor` | `CONTRIBUINDO.md`; [Control flow](https://angular.dev/guide/templates/control-flow) |
| Classe e estilo no template | `[class.x]` e `[style.x]`, nunca `NgClass`/`NgStyle` | [Style guide: prefer class and style](https://angular.dev/style-guide#prefer-class-and-style-over-ngclass-and-ngstyle) |
| HTTP | só o `ApiService` (`core/api.service.ts`); a exceção registrada é o `auth.service.ts`. Erro da API vira texto com `mensagemDeErro` (`core/erro.interceptor.ts`) | `CLAUDE.md`; regra 1 do teste de arquitetura |
| Quem importa quem | `core` não importa `compartilhado`, `layout` nem `paginas`; `compartilhado` não importa `paginas` nem `layout`; página com rota não importa outra página com rota; `compartilhado/ui` não importa `bancos`, `lancamentos` nem `glossario`; a lógica da tela (`paginas/<grupo>/<tela>.ts`) não importa Angular, rxjs nem o `ApiService` | `testes/arquitetura/arquitetura.spec.ts` (regras 2 a 7; só local, #476, #634, ADR 0059) |
| Rotas | `loadComponent` em toda tela, `title: '<Tela> · Finanças Local'` | `app.routes.ts` |
| Formulário | template-driven com `ngModel` (25 arquivos); sem `ReactiveFormsModule` | prática do código |
| Inscrição em Observable | `.subscribe()` do `ApiService` direto; o que vive mais que a chamada usa `takeUntilDestroyed` | prática do código |
| Dinheiro na tela | `MoedaPipe`, que se registra com o nome `currency` (`compartilhado/moeda.pipe.ts`): respeita "ocultar valores" e usa o "−" de verdade | `docs/UI-UX.md` ("ocultar valores"); #257 |
| Preferência de exibição | `localStorage` com chave `financas.*`, lida com `try`/`catch` | ADR 0017 |
| Comentário | em português; o JSDoc da classe diz a issue, a ADR e o princípio de UI/UX que a tela segue (`/** #173 (ADR 0045): ... (Shneiderman: overview first) */`) | prática do código |
| Na dúvida | siga a forma do arquivo em que está | [Style guide: when in doubt, prefer consistency](https://angular.dev/style-guide#when-in-doubt-prefer-consistency) |

## Onde divergimos dos guias de fora, e por quê

O angular.dev, os guias da comunidade e o hábito de quem vem de outro projeto Angular sugerem coisas que o projeto decidiu **não** fazer. Ao ver uma sugestão destas num guia ou num revisor, a decisão do projeto vale.

| O guia ou o hábito diz | Aqui | Por quê |
|---|---|---|
| OnPush e zoneless, o padrão do Angular 22 ([Zoneless](https://angular.dev/guide/zoneless), [Skipping component subtrees](https://angular.dev/best-practices/skipping-subtrees)) | `Eager` com zone.js | muitas telas guardam estado em campo comum; com OnPush elas param de atualizar sem erro (`CLAUDE.md`) |
| Template e CSS em arquivo próprio, com o mesmo nome do `.ts` ([style guide](https://angular.dev/style-guide#use-the-same-file-name-for-a-components-typescript-template-and-styles)) | inline até 150 linhas de template | ADR 0059: acima disso, arquivo próprio, mas só depois do pré-requisito dela; até lá, não mude numa tela só |
| Arquivo sem sufixo (`user-profile.ts`) | `xxx.component.ts`, `xxx.pipe.ts`, `xxx.directive.ts`, `xxx.service.ts`; função pura em `xxx.ts` | consistência com os 85 arquivos de hoje |
| Pastas por funcionalidade ([style guide](https://angular.dev/style-guide#organize-your-project-by-feature-areas)) | `core/`, `compartilhado/` (`ui/` e por assunto), `layout/`, `paginas/<grupo do menu>/` | ADR 0058, `CLAUDE.md` e o teste de arquitetura, que conferem quem importa quem por pasta |
| Teste ao lado do código ([style guide](https://angular.dev/style-guide#group-closely-related-files-together-in-the-same-directory)) | `testes/jest/regressao`, `testes/jest/usabilidade`, `testes/navegador/` | ADR 0013 e ADR 0041: a pasta diz o tipo do teste e a CI escolhe por ela |
| `protected` no que só o template usa ([style guide](https://angular.dev/style-guide#use-protected-on-class-members-that-are-only-used-by-a-components-template)) | público (0 `protected`) | *Inferido:* os testes do Jest leem e chamam membros do componente direto. Não troque sem combinar |
| `HttpClient` no serviço de cada assunto | só o `ApiService` | `CLAUDE.md`, teste de arquitetura |
| NgRx, Angular Material, CDK, biblioteca de componentes | nada; componentes próprios em `compartilhado/` | ADR 0004 e 0034 (nada de terceiros, CSP), ADR 0021 e 0032 (componentes padrão); dependência nova pede ADR e medida de tempo no Jest (`CONTRIBUINDO.md`) |
| `TestBed` para tudo | cálculo em função pura, testado sem `TestBed`; `TestBed` só para o comportamento e os atributos de um componente (66 de 111 arquivos do Jest); desenho, CSS e rota no navegador | #204: o Jest chegou a 6 min; trava de 20 s por arquivo (`CONTRIBUINDO.md`, "Testes rápidos") |
| `CurrencyPipe` do Angular | `MoedaPipe` (nome `currency`) | importar o `CurrencyPipe` de `@angular/common` passa por cima do "ocultar valores" sem erro |
| `title=` para dica ou definição | `appDica` (`compartilhado/ui/dica.directive.ts`) ou `<app-termo>` | `title=` não aparece no toque nem no foco do teclado (WCAG 1.4.13; `docs/UI-UX.md`) |
| Identificadores e textos em inglês | português | `CLAUDE.md` |
