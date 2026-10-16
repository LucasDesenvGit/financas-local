# 0005. Segredos criptografados no banco com ASP.NET Data Protection

- **Status:** Aceita
- **Data:** 2026-10-15
- **Relacionadas:** [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md)

## Contexto

O sistema precisa guardar credenciais de serviços externos — o Client Secret da Pluggy (acesso aos dados bancários via Open Finance) e o token opcional da brapi. Elas são informadas pelo usuário na tela de configurações, não na instalação, e não podem vazar por arquivo de configuração, backup do repositório nem resposta da API.

## Alternativas consideradas

- **`appsettings.json` ou variável de ambiente** — simples; mas o segredo fica em texto puro no disco, e o usuário teria de editar arquivo em vez de usar a tela.
- **Cofre do sistema operacional** (Credential Manager, libsecret/keyring) — forte; mas cada sistema tem uma API diferente, e o serviço do systemd `--user` nem sempre tem acesso ao keyring da sessão.
- **ASP.NET Data Protection + tabela `Configuracao`** — o valor é criptografado pela API e guardado no banco; as chaves ficam em `dados/chaves`.

## Decisão

Guardamos os segredos criptografados com ASP.NET Data Protection na tabela `Configuracao` (coluna `Protegido`). As chaves ficam em `dados/chaves`, protegidas por DPAPI no Windows e por permissão de arquivo no Linux. A API nunca devolve o valor de um segredo.

## Consequências

- **Positivas:** o segredo é configurado pela tela; uma cópia do banco sozinha não revela nada; a mesma solução nos dois sistemas.
- **Negativas:**
  - no Linux a proteção das chaves é só a permissão de arquivo — quem lê `dados/` com a conta do serviço lê as chaves;
  - os segredos ficam presos às chaves daquela instalação: ao trocar de máquina ou sistema, ou ao migrar com `Financas.MigrarSqlite`, eles precisam ser informados de novo;
  - perder `dados/chaves` torna os segredos ilegíveis (o backup do banco não basta).
- **Na prática:** segredo novo usa `ConfiguracaoService.DefinirAsync(chave, valor, protegido: true)`; nunca em `appsettings`, log ou DTO de resposta. Em desenvolvimento as chaves ficam em `dados/chaves-dev`.
