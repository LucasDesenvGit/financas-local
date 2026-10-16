# Finanças Local

Sistema de gerenciamento financeiro pessoal que roda inteiro no seu computador, Windows ou Linux. A API é em C# (.NET 10) e hospeda a si mesma via Kestrel (sem IIS), o front-end é em Angular e o banco é SQLite. Tudo fica numa pasta local só sua. Os dados bancários chegam automaticamente pelo **Open Finance**, via **Meu Pluggy**.

| Documento | Conteúdo |
|---|---|
| [docs/ARQUITETURA.md](docs/ARQUITETURA.md) | Camadas, pastas e decisões técnicas |
| [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md) | Como alterar a estrutura do banco (sem migrations) |
| [docs/adr/](docs/adr/README.md) | Decisões de arquitetura (ADRs): por que cada escolha foi feita |
| [CONTRIBUINDO.md](CONTRIBUINDO.md) | Branches, commits, pull requests e checklist |
| [CHANGELOG.md](CHANGELOG.md) | Histórico de versões |

## O que tem

| Área | Funcionalidades |
|---|---|
| Painel | Patrimônio líquido (com a renda variável pela cotação atual), patrimônio sem renda variável, saldo em contas, faturas, investimentos, receitas × despesas do mês, gastos por categoria, evolução de 12 meses, orçamentos, próximos vencimentos e alertas (orçamento estourado, conta vencida, consentimento expirando, falha de sincronização) |
| Transações | Filtros por período, conta, categoria, tipo e texto; categorização em lote; lançamentos manuais; transferências; parcelas; transações pendentes; opção de ignorar nos relatórios |
| Contas e cartões | Contas do Open Finance (automáticas) e manuais (carteira, bancos sem Open Finance); limite e faturas de cartão |
| Orçamentos | Limite mensal por categoria, progresso, "copiar do mês anterior" |
| Metas | Valor alvo, data, aporte mensal sugerido, metas que acompanham o saldo de uma conta |
| Contas a pagar | Vencimentos únicos, mensais ou anuais, com lembrete no painel |
| Investimentos | Carteira de renda variável (FIIs, ações, ETFs, BDRs) com preço médio, cotação atual, valorização de cada compra, proventos e DY; comparação com a Selic (carteira, cada ativo e cada investimento do Open Finance); distribuição por classe |
| Relatórios | Fluxo de caixa mensal com saldo acumulado, despesas/receitas por categoria, maiores despesas |
| Categorias e regras | Categorias com subcategorias, regras automáticas ("descrição contém UBER → Transporte"), mapeamento das categorias do banco |
| Emergências | Marque despesas como emergência (na transação ou em lote). Orçamentos e relatórios mostram em laranja quando o limite ou o mês só estourou por causa delas |
| Sugestões de categoria | Em Transações › Sugestões de categoria: agrupa transações parecidas, sugere a categoria pelo seu histórico e por palavras-chave e cria regras com um clique |
| Calculadora | Botão flutuante no canto superior direito (atalho Alt+C) |

## Instalação (uma vez)

Abra um terminal com bash disponível (no Linux, o terminal do próprio sistema; no Windows, o Git Bash ou o WSL).

```bash
cd financas-local/scripts
./1-preparar-ambiente.sh     # .NET 10 SDK, Node.js e sqlite3 (poucos minutos)
./2-criar-banco.sh           # cria o banco SQLite com a estrutura completa
./3-publicar.sh              # compila e publica como serviço do sistema
```

Depois, abra **https://localhost:5443**. No primeiro acesso você cria o usuário e a senha (mínimo de 8 caracteres).

### De onde vem cada coisa

| Componente | Origem | Verificação |
|---|---|---|
| .NET 10 SDK (portátil em `ferramentas/dotnet`) | metadados oficiais `builds.dotnet.microsoft.com` | SHA-512 + assinatura do instalador |
| Node.js LTS (portátil em `ferramentas/node`, usado só para compilar o Angular) | `nodejs.org` | SHA-256 do `SHASUMS256.txt` oficial |
| `sqlite3` (CLI) | gerenciador de pacotes do sistema operacional | — |

`1-preparar-ambiente.sh` verifica cada um desses componentes e só baixa/instala o que ainda não estiver disponível.

## Migrar de uma instalação com SQL Server

Se você já usava o Finanças Local no Windows com IIS e SQL Server Express, use a ferramenta de migração para trazer os dados para o novo banco SQLite. **Não rode o `2-criar-banco.sh` antes**: os dados iniciais dele (categorias e configurações padrão) conflitam com os dados migrados. O destino precisa ser um banco vazio, só com a estrutura:

```bash
# 1. Pare o site antigo no IIS, para nada mudar no SQL Server durante a cópia.
# 2. Crie o banco vazio, só com a estrutura:
sqlite3 dados/sql/FinancasLocal.db < banco/01-estrutura.sql
# 3. Migre (a connection string antiga está no appsettings.Production.json da publicação do IIS):
dotnet run --project src/backend/Financas.MigrarSqlite -- \
  'Server=.\FINANCAS;Database=FinancasLocal;Integrated Security=True;Encrypt=True;TrustServerCertificate=True' \
  dados/sql/FinancasLocal.db
# 4. Publique normalmente:
scripts/3-publicar.sh
```

A ferramenta não altera o SQL Server (só lê dele) e confere a contagem de linhas de cada tabela no final. A pasta `dados\sql` da instalação antiga só é acessível por Administradores, então no Windows rode esses comandos num Git Bash aberto como Administrador.

É seguro rodar mesmo sem um SQL Server disponível: a ferramenta detecta que não há nada para migrar e termina sem erro. Como o Client Secret da Pluggy e o token da brapi foram criptografados com chaves da instalação antiga (que não podem ser lidas em outra máquina ou sistema operacional), a migração limpa esses dois campos — informe-os de novo em **Configurações** depois de migrar (mesma situação da linha "o secret da Pluggy não abre mais" em Problemas comuns).

## Conectar o Open Finance (Meu Pluggy)

No Brasil, só instituições autorizadas pelo Banco Central podem consumir as APIs do Open Finance diretamente. Por isso o sistema usa o **Meu Pluggy**, que é gratuito para os dados do seu próprio CPF.

1. Em **meu.pluggy.ai**, crie a conta e conecte seus bancos. O consentimento é dado no app do banco.
2. Em **dashboard.pluggy.ai**, crie uma conta de desenvolvedor, adicione o conector *MeuPluggy* e crie uma aplicação de desenvolvimento. Copie o **Client ID** e o **Client Secret**.
3. Ainda no dashboard, abra a aplicação de demonstração, conecte o *MeuPluggy* e autorize **uma vez por banco**. Cada autorização gera um **Item ID**.
4. No sistema, vá em **Configurações → Open Finance**: informe o Client ID e o Secret, clique em **Testar conexão** e adicione cada Item ID em **Conexões**.

A primeira carga importa os últimos 365 dias. Depois disso, a API sincroniza sozinha a cada 6 horas (o intervalo é configurável) e relê os últimos 30 dias para pegar transações que mudaram. O Meu Pluggy atualiza os dados nos bancos uma vez por dia.

Pontos de atenção:
- O consentimento do Open Finance vale no máximo 12 meses. O painel avisa 30 dias antes de expirar, e a renovação é feita no Meu Pluggy.
- A integração usa o endpoint novo `/v2/transactions`, com paginação por cursor. O endpoint antigo deixa de funcionar em 31/12/2026.

## Carteira de renda variável

As operações de renda variável entram na carteira de três formas:

1. **Corretora conectada no Meu Pluggy:** na sincronização, FIIs, ações e ETFs informados pela corretora viram ativos com as compras e vendas (ou, se a corretora só informar a posição, uma compra única com o custo informado).
2. **Importar do extrato:** procura códigos da B3 nas descrições das transações ("COMPRA MXRF11 10 x 9,85"). Quando faltam quantidade ou preço, eles são calculados pelo valor da transação e pela cotação de fechamento do dia. Você confere e escolhe o que importar.
3. **Lançamento manual** em **+ Lançar compra/venda**. Ativos com lançamentos manuais não recebem importação do Open Finance, para não duplicar.

A aba **FIIs** mostra renda mensal estimada, DY e yield on cost, cada compra comparada com a cotação e a renda de hoje, proventos por mês, evolução do patrimônio, rentabilidade x Selic x IFIX, cotação x suas compras e a distribuição por segmento. Créditos de rendimento encontrados no extrato aparecem como "recebido no extrato".

- O valor das cotas pela cotação atual entra no patrimônio líquido. O card "Patrimônio sem renda variável" mostra o mesmo número sem esses ativos.
- Cotações e proventos vêm do Yahoo Finance (fonte gratuita e não oficial). Se um ativo não for encontrado, informe a cotação manualmente em "editar". Proventos são estimados pela quantidade em carteira na data-com.
- Se um investimento do Open Finance tiver o mesmo código de um ativo lançado na carteira, ele não é somado duas vezes.
- As transferências para as principais corretoras são categorizadas como "Investimentos" (use **Reaplicar regras** para as antigas).

## Segurança

Como tudo é local, o objetivo foi manter o sistema simples sem abrir mão do essencial:

- **Sem exposição na rede.** O Kestrel (servidor web embutido na própria API, sem IIS nem proxy reverso) responde somente em `127.0.0.1`, via HTTPS. O SQLite é só um arquivo local: não tem porta nem listener de rede para proteger.
- **Sem senhas em arquivos.** O SQLite não tem conceito de login/usuário de banco — o controle de acesso é só a permissão do sistema operacional sobre a pasta `dados`: lê e grava quem roda o processo da API, ou seja, a conta do serviço do sistema (systemd no Linux, Serviço do Windows no Windows).
- **Client Secret da Pluggy criptografado** no banco com ASP.NET Data Protection. As chaves ficam em `dados\chaves`: no Windows, protegidas por DPAPI e acessíveis só por Administradores, SYSTEM e a conta do serviço; no Linux, protegidas pelas permissões do sistema de arquivos sobre essa pasta. A API nunca devolve o secret.
- **Login:** senha com hash PBKDF2-SHA512 (ASP.NET Identity), bloqueio de 15 min após 5 erros, limite de 5 tentativas por minuto e tempo de resposta igual para usuário inexistente.
- **Sessão:** cookie `__Host-` HttpOnly + Secure + SameSite=Strict, que expira em 2 h sem uso. Trocar a senha derruba as outras sessões.
- **CSRF:** token antifalsificação validado em todo POST, PUT, PATCH e DELETE.
- **Cabeçalhos de segurança:** CSP restritiva (sem scripts de terceiros; nada é carregado da internet), X-Frame-Options, nosniff, Referrer-Policy e `no-store` nas respostas da API.
- **Validação de host:** só `localhost`/`127.0.0.1`, o que protege contra DNS rebinding.
- **Auditoria** de logins, falhas, trocas de senha e alterações de credenciais, visível em Configurações.
- Consultas ao banco somente via LINQ/EF Core, sem SQL montado a partir de texto do usuário.
- **Saídas para a internet** (todas HTTPS): `api.pluggy.ai` (Open Finance), `api.bcb.gov.br` (taxa Selic, só datas são enviadas) e `query1.finance.yahoo.com` / `brapi.dev` (cotações, só o código do ativo é enviado). Quantidades, valores e dados pessoais não saem do computador. O token opcional da brapi fica criptografado como o secret da Pluggy.

Recomendações:
- Ative criptografia de disco (**BitLocker** no Windows, **LUKS**/dm-crypt no Linux), porque o SQLite não criptografa o arquivo do banco.
- Agende execuções periódicas de `scripts/4-backup.sh` e copie de vez em quando a pasta `backups` para um disco externo.
- Rode o `1-preparar-ambiente.sh` a cada poucos meses para atualizar o .NET com as correções de segurança.

## Atualizar o sistema depois de mudar o código

```bash
scripts/3-publicar.sh
```

A publicação para o serviço do sistema por alguns segundos, copia os arquivos novos e inicia o serviço de novo. Nenhum dado é perdido.

**Banco:** não há migrations. Toda a estrutura está em `banco\01-estrutura.sql`, que é idempotente. Para mudar o banco, altere o script, rode `2-criar-banco.sh` e ajuste as entidades e o `FinancasDbContext`.

## Desenvolvimento (Visual Studio)

- Abra `FinancasLocal.sln`. O perfil *Desenvolvimento (Kestrel)* sobe a API em `https://localhost:7443`. Em desenvolvimento, as chaves ficam em `dados\chaves-dev`, então o Client Secret precisa ser informado de novo.
- No front-end, rode `npm start` em `src\frontend`. O Angular abre em `http://localhost:4200` e redireciona `/api` para a API.
- Antes, configure o PATH para as ferramentas portáteis (ou use as suas):
  `$env:PATH = "D:\FinancasLocal\ferramentas\dotnet;D:\FinancasLocal\ferramentas\node;$env:PATH"`

## Estrutura

```
financas-local/
├── .github/               CI, templates de PR/issue, CODEOWNERS, Dependabot
├── banco/                 scripts SQL (estrutura, dados iniciais)
├── docs/                  arquitetura, guia do banco e ADRs (docs/adr)
├── scripts/               instalação, publicação e backup (bash, Windows e Linux)
│   └── git/              configuração do repositório no GitHub (PowerShell)
├── src/backend/           Financas.Api (C# / .NET 10 / EF Core) e Financas.MigrarSqlite (migração do SQL Server)
├── src/frontend/          financas-web (Angular)
├── dados/                 [gerado] banco (SQLite), chaves, logs — acesso restrito
├── backups/               [gerado] backups do banco
├── publicado/             [gerado] aplicação publicada
├── ferramentas/           [gerado] .NET SDK, Node.js, caches NuGet/npm
└── instaladores/          [gerado] instaladores baixados (podem ser apagados)
```

## Problemas comuns

| Sintoma | O que fazer |
|---|---|
| Navegador alerta sobre o certificado | Rode o `3-publicar.sh` de novo: ele recoloca o certificado nos confiáveis. Firefox: em `about:config`, ative `security.enterprise_roots.enabled` |
| Sincronização falha com 401/403 | Confira o Client ID e o Secret e se o MeuPluggy foi autorizado para aquele banco |
| O secret da Pluggy não abre mais (depois de migrar com o `Financas.MigrarSqlite`, trocar de máquina/SO ou apagar `dados\chaves`) | As chaves de proteção de dados são específicas da instalação: informe o Client Secret de novo em Configurações |
