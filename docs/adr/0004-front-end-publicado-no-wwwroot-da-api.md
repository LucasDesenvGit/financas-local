# 0004. Front-end publicado dentro da API (mesma origem)

- **Status:** Aceita
- **Data:** 2026-10-15
- **Relacionadas:** [0005](0005-segredos-criptografados-com-data-protection.md)

## Contexto

O front-end é uma aplicação Angular que conversa com a API em C#. Tudo roda na máquina do usuário e guarda dados financeiros, então a sessão precisa ser protegida contra roubo de cookie, CSRF e scripts de terceiros, com o mínimo de configuração para quem instala.

## Alternativas consideradas

- **Front-end e API em origens separadas** (ex.: servidor estático em outra porta) — independência de publicação; mas exige CORS, cookies de sessão entre origens (`SameSite=None`) e dois processos para manter no ar.
- **Front-end servido pela própria API** (`wwwroot`) — uma origem só.

## Decisão

Publicamos o build do Angular dentro do `wwwroot` da API: site e API têm a mesma origem, e o `MapFallbackToFile("index.html")` atende as rotas do Angular.

## Consequências

- **Positivas:** sem CORS; cookies com prefixo `__Host-`, `HttpOnly`, `Secure` e `SameSite=Strict`; CSRF resolvido com o par de cookies/cabeçalho `X-XSRF-TOKEN` que o `HttpClient` do Angular já envia; CSP restritiva possível porque nada vem de outra origem; um único processo e um único certificado.
- **Negativas:** front e API são publicados juntos (`scripts/3-publicar-atualizar.sh` compila os dois); em desenvolvimento é preciso o proxy do `npm start` (`/api` → API) para simular a mesma origem.
- **Na prática:** o front-end chama a API só pelo `ApiService`, com caminhos relativos `/api/...`; nenhum script, fonte ou recurso de terceiros pode ser carregado no front-end.
