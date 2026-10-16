# 0015. Parâmetros do sistema no banco, editáveis pelo site

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois da decisão, no PR #28)
- **Relacionadas:** [0005](0005-segredos-criptografados-com-data-protection.md) (segredos na tabela `Configuracao`)

## Contexto

Ajustes do dia a dia estavam espalhados: intervalo e dias da sincronização na tela do Open Finance, dias de "próximos vencimentos" e de aviso de consentimento fixos no código, tentativas de login e tempo de bloqueio no `appsettings.json`. Mudar qualquer um exigia editar arquivo e reiniciar o serviço — inviável para quem só usa o site (issue #18).

## Alternativas consideradas

- **Continuar no `appsettings.json`** — padrão do ASP.NET; mas pede acesso ao arquivo, reinício do serviço e não tem validação na hora.
- **Tabela nova de parâmetros** — tipada; mas duplicaria a tabela chave-valor `Configuracao`, que já existe e já guarda os segredos.
- **Tabela `Configuracao` + catálogo no código** — reaproveita o que existe; a tela e a validação saem de uma lista só.

## Decisão

Parâmetros que a pessoa pode querer mudar ficam na tabela `Configuracao` e são editados na aba **Configurações › Parametrização** (`GET/PUT /api/parametros`). O catálogo — nome, descrição, faixa, padrão e unidade — fica em `ParametroService.Catalogo`; a tela e a validação são geradas dele. O `PUT` valida tudo antes de gravar (todos ou nenhum), recusa chave desconhecida e registra `ParametrosAlterados` na auditoria. Os valores valem na hora, sem reiniciar. Quando o `appsettings.json` já tinha valor (tentativas/bloqueio de login), ele vira o padrão.

Ficam **fora**, de propósito: segredos (fluxo criptografado próprio, ADR 0005); caminho do banco, pasta das chaves, porta e certificado (precisam existir antes de o banco abrir); tempo de sessão (configurado na inicialização do cookie).

## Consequências

- **Positivas:** ajuste sem editar arquivo nem reiniciar; faixa válida e padrão visíveis na tela; parâmetro novo é uma linha no catálogo; alterações auditadas.
- **Negativas:**
  - parâmetros de **segurança** (tentativas e bloqueio de login) passam a ser alteráveis por quem estiver logado — alguém com a sessão aberta pode afrouxar a proteção contra força bruta (mitigado pela faixa mínima de 3 tentativas e pela auditoria);
  - duas fontes para alguns valores (banco e, como padrão, `appsettings.json`), o que pode confundir quem procura o valor em vigor;
  - os valores ficam no banco: restaurar um backup antigo volta também a parametrização.
- **Na prática:** configuração nova que a pessoa pode mudar → entrada em `ParametroService.Catalogo` (com faixa e padrão), lida pelo serviço na hora de usar (não em cache na inicialização). Configuração de infraestrutura continua no `appsettings.json`.
