# 0034. Build do front-end sem SRI, para manter a CSP sem script inline

- **Status:** Aceita
- **Data:** 2026-10-19
- **Relacionadas:** [0004](0004-front-end-publicado-no-wwwroot-da-api.md)

## Contexto

A API manda a CSP `script-src 'self'`, sem `'unsafe-inline'` (`CabecalhosSegurancaMiddleware`, em `Seguranca/Seguranca.cs`). Isso só é possível porque o site e a API têm a mesma origem e nada vem de fora (ADR 0004).

O build de produção do Angular tinha `"subresourceIntegrity": true` desde o projeto Angular 20. O SRI põe o hash `sha384` de cada arquivo no `index.html`, para o navegador recusar um arquivo adulterado. Com o `@angular/build` 22, o hash dos chunks carregados sob demanda (as páginas lazy) passou a ir num `<script type="importmap">` escrito dentro do `index.html` publicado (#157). A CSP bloqueia esse script inline:

- o console mostra erro de CSP em toda página;
- o navegador ignora o import map, então o SRI valia só para `main`, `polyfills` e o CSS, e não para as páginas lazy.

O site continuava funcionando, porque os chunks são da mesma origem.

O SRI protege contra arquivo trocado no caminho entre outro servidor (uma CDN) e o navegador. Aqui não há outro servidor: o `index.html` e os chunks saem do mesmo Kestrel local, da mesma pasta `wwwroot`. Quem conseguisse trocar um chunk no disco trocaria o `index.html` junto, com os hashes novos.

## Alternativas consideradas

- **Desligar o SRI.** Prós: some o script inline e a CSP fica exatamente igual; nenhuma mudança na API. Contras: perde-se a conferência de integridade, que aqui não protegia contra nenhum ataque realista *(julgamento próprio)*.
- **Pôr o hash do import map na CSP** (a API calcula o `sha256` do script inline do `wwwroot/index.html` ao iniciar e acrescenta `'sha256-…'` ao `script-src`). Prós: SRI completo. Contras: o hash muda a cada build; a CSP passa a depender do conteúdo do `index.html`, e um erro nessa leitura derruba o site ou afrouxa a CSP; mais código de segurança para manter.
- **`"security": { "autoCsp": true }` do Angular.** Põe uma CSP em `<meta>`, mas a do cabeçalho da API continua valendo junto e seguiria bloqueando. Só funcionaria mudando a API também.
- **`'unsafe-inline'` em `script-src`.** Descartada: tira a principal defesa da CSP contra script injetado.

## Decisão

O build de produção do front-end não usa SRI (`"subresourceIntegrity": false` no `angular.json`). Com isso o `index.html` publicado não tem script inline, e a CSP continua `script-src 'self'`.

## Consequências

- **Positivas:** console sem erro de CSP; a CSP fica como estava, sem depender do conteúdo do build; nenhum código novo na API.
- **Negativas:** o navegador não confere mais o hash de `main`, `polyfills` e do CSS (os chunks lazy já não eram conferidos). Se um dia o site passar a carregar arquivo de outra origem, isso precisa ser revisto (e a ADR 0004 já proíbe).
- **Na prática:** o teste `testes/regressao/csp-sem-script-inline.spec.ts` falha se o `angular.json` voltar a ligar o `subresourceIntegrity` ou se o `src/index.html` ganhar um `<script>` com conteúdo. Uma opção nova do build que gere script inline precisa de hash na CSP ou de outra saída registrada aqui.
