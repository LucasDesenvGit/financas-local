# Finanças Local

Sistema de gerenciamento financeiro pessoal que roda inteiro no seu computador. A API é em C# (.NET 10) e roda no IIS, o front-end é em Angular e o banco é SQL Server Express. Tudo fica em `D:\FinancasLocal`. Os dados bancários chegam automaticamente pelo **Open Finance**, via **Meu Pluggy**.

| Documento | Conteúdo |
|---|---|
| [docs/ARQUITETURA.md](docs/ARQUITETURA.md) | Camadas, pastas e decisões técnicas |
| [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md) | Como alterar a estrutura do banco (sem migrations) |
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

Abra o **Windows PowerShell como Administrador**. Use o "Windows PowerShell", não o PowerShell 7.

```powershell
Set-ExecutionPolicy -Scope Process Bypass
cd D:\FinancasLocal\scripts
.\1-instalar-ambiente.ps1     # IIS, .NET 10, Node.js portátil, SQL Server 2025 Express (15-30 min)
.\2-criar-banco.ps1           # cria o banco com a estrutura completa
.\3-publicar.ps1              # compila e publica no IIS
```

Depois, abra **https://localhost:5443**. No primeiro acesso você cria o usuário e a senha (mínimo de 8 caracteres).

### De onde vem cada coisa

| Componente | Origem | Verificação |
|---|---|---|
| IIS | Recurso do próprio Windows | — |
| ASP.NET Core Hosting Bundle 10 | metadados oficiais `builds.dotnet.microsoft.com` | SHA-512 publicado pela Microsoft + assinatura digital Microsoft |
| .NET 10 SDK (portátil em `ferramentas\dotnet`) | idem | SHA-512 + assinatura do `dotnet.exe` |
| SQL Server 2025 Express | `download.microsoft.com` | assinatura digital Microsoft (instalador e `setup.exe`) |
| Node.js LTS (portátil em `ferramentas\node`, usado só para compilar o Angular) | `nodejs.org` | SHA-256 do `SHASUMS256.txt` oficial |

Se o download do SQL Server falhar (a Microsoft às vezes muda o link), baixe manualmente em https://www.microsoft.com/sql-server/sql-server-downloads (Express → *Download Media* → *Express Core*) e rode:

```powershell
.\1-instalar-ambiente.ps1 -InstaladorSql "C:\caminho\SQLEXPR_x64_ENU.exe"
```

**Já tem um SQL Server?** Rode `.\1-instalar-ambiente.ps1 -PularSqlServer` e informe a instância nos outros scripts, por exemplo `-InstanciaSql "localhost\SQLEXPRESS"`.

**O que inevitavelmente fica no C:** o IIS e o Hosting Bundle (são componentes do Windows) e, se você já tiver outro SQL Server instalado, os componentes compartilhados dele. Dados, banco, backups, chaves, SDK, Node, pacotes NuGet e npm ficam todos no D:.

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

- **Sem exposição na rede.** O site responde somente em `127.0.0.1` e via HTTPS. O SQL Server foi instalado com TCP/IP e Named Pipes **desligados** e só aceita conexões locais.
- **Sem senhas em arquivos.** A API acessa o banco com a identidade do pool do IIS (`IIS APPPOOL\FinancasLocal`, autenticação do Windows). Esse usuário só pode ler e gravar dados: não altera a estrutura, não cria logins e não apaga a auditoria. O SQL Server não tem login `sa`.
- **Client Secret da Pluggy criptografado** no banco com ASP.NET Data Protection. As chaves ficam em `dados\chaves`, protegidas por DPAPI e acessíveis só por Administradores, SYSTEM e o pool do IIS. A API nunca devolve o secret.
- **Login:** senha com hash PBKDF2-SHA512 (ASP.NET Identity), bloqueio de 15 min após 5 erros, limite de 5 tentativas por minuto e tempo de resposta igual para usuário inexistente.
- **Sessão:** cookie `__Host-` HttpOnly + Secure + SameSite=Strict, que expira em 2 h sem uso. Trocar a senha derruba as outras sessões.
- **CSRF:** token antifalsificação validado em todo POST, PUT, PATCH e DELETE.
- **Cabeçalhos de segurança:** CSP restritiva (sem scripts de terceiros; nada é carregado da internet), X-Frame-Options, nosniff, Referrer-Policy e `no-store` nas respostas da API.
- **Validação de host:** só `localhost`/`127.0.0.1`, o que protege contra DNS rebinding.
- **Auditoria** de logins, falhas, trocas de senha e alterações de credenciais, visível em Configurações.
- Consultas ao banco somente via LINQ/EF Core, sem SQL montado a partir de texto do usuário.
- **Saídas para a internet** (todas HTTPS): `api.pluggy.ai` (Open Finance), `api.bcb.gov.br` (taxa Selic, só datas são enviadas) e `query1.finance.yahoo.com` / `brapi.dev` (cotações, só o código do ativo é enviado). Quantidades, valores e dados pessoais não saem do computador. O token opcional da brapi fica criptografado como o secret da Pluggy.

Recomendações:
- Ative o **BitLocker** no disco D:, porque o SQL Server Express não criptografa os arquivos do banco.
- Agende o backup com `.\4-backup.ps1 -Agendar` e copie de vez em quando a pasta `backups` para um disco externo.
- Rode o `1-instalar-ambiente.ps1` a cada poucos meses para atualizar o .NET com as correções de segurança.

## Atualizar o sistema depois de mudar o código

```powershell
D:\FinancasLocal\scripts\3-publicar.ps1
```

A publicação coloca o site em manutenção por alguns segundos (`app_offline.htm`), copia os arquivos e volta. Nenhum dado é perdido.

**Banco:** não há migrations. Toda a estrutura está em `banco\01-estrutura.sql`, que é idempotente. Para mudar o banco, altere o script, rode `2-criar-banco.ps1` e ajuste as entidades e o `FinancasDbContext`.

## Desenvolvimento (Visual Studio)

- Abra `FinancasLocal.sln`. O perfil *Desenvolvimento (Kestrel)* sobe a API em `https://localhost:7443`. Em desenvolvimento, as chaves ficam em `dados\chaves-dev`, então o Client Secret precisa ser informado de novo.
- No front-end, rode `npm start` em `src\frontend`. O Angular abre em `http://localhost:4200` e redireciona `/api` para a API.
- Antes, configure o PATH para as ferramentas portáteis (ou use as suas):
  `$env:PATH = "D:\FinancasLocal\ferramentas\dotnet;D:\FinancasLocal\ferramentas\node;$env:PATH"`

## Estrutura

```
D:\FinancasLocal
├── .github\               CI, templates de PR/issue, CODEOWNERS, Dependabot
├── banco\                 scripts SQL (estrutura, dados iniciais, permissões)
├── docs\                  arquitetura e guia do banco
├── scripts\               instalação, publicação e backup (PowerShell)
│   └── git\              configuração do repositório no GitHub
├── src\backend\           Financas.Api (C# / .NET 10 / EF Core)
├── src\frontend\          financas-web (Angular)
├── dados\                 [gerado] banco (sql), chaves, logs do IIS — acesso restrito
├── backups\               [gerado] backups .bak
├── publicado\             [gerado] site publicado no IIS
├── ferramentas\           [gerado] .NET SDK, Node.js, caches NuGet/npm
└── instaladores\          [gerado] instaladores baixados (podem ser apagados)
```

## Problemas comuns

| Sintoma | O que fazer |
|---|---|
| Erro 500.19 ou 500.30 no navegador | Rode `iisreset` e confira se o Hosting Bundle foi instalado **depois** do IIS (o script 1 já segue essa ordem). Detalhes em: Visualizador de Eventos → Aplicativo |
| "Login failed for user IIS APPPOOL\FinancasLocal" | Rode novamente o `2-criar-banco.ps1` |
| Navegador alerta sobre o certificado | Rode o `3-publicar.ps1` de novo: ele recoloca o certificado nos confiáveis. Firefox: em `about:config`, ative `security.enterprise_roots.enabled` |
| Sincronização falha com 401/403 | Confira o Client ID e o Secret e se o MeuPluggy foi autorizado para aquele banco |
| Depois de reinstalar o Windows ou mudar o pool, o secret não abre | As chaves DPAPI são da máquina/conta: informe o Client Secret de novo em Configurações |
