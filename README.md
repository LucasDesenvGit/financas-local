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
| Painel | Em duas seções. **Situação hoje** (não muda com o mês): patrimônio líquido (com a renda variável pela cotação atual), patrimônio sem renda variável, saldo em contas, dívida total nos cartões, investimentos e próximos vencimentos. **Mês de …** (segue o seletor): receitas × despesas, resultado, **faturas do mês** por cartão (só o que foi lançado no mês — compras e a parcela do mês, sem parcelas futuras —, com a fatura que vence no mês e se está paga, fechada ou em aberto, e link para as faturas do cartão), gastos por categoria e orçamentos. Evolução de 12 meses e últimas transações em "Mais análises". Alertas (orçamento estourado, conta vencida, consentimento expirando, falha de sincronização) |
| Transações | **Tipo de cada lançamento** em cada linha, com ícone e texto (compra no cartão, Pix enviado ou recebido, entre suas contas, pagamento de fatura, boleto, débito, saque, tarifa, rendimento, salário…), vindo do Open Finance ou reconhecido pela descrição, com o aviso "somando nos gastos" quando um pagamento de fatura conta em dobro; filtros por período, conta, uma ou várias categorias (com "sem categoria"), tipo, lançamento, texto e **descrição pouco informativa** ("PIX ENVIADO", "PAGAMENTO", só o nome do banco — com "marcar como revisada" e aviso discreto no Painel); categorização em lote; **modo edição** de categorias (troca direto na lista, "Salvar alterações", todas as do filtro de uma vez, "Desfazer" e sugestão de regra); lançamentos manuais; transferências; parcelas; transações pendentes; opção de ignorar nos relatórios |
| Contas e cartões | Contas do Open Finance (automáticas) e manuais (carteira, bancos sem Open Finance), cada uma com o **selo do banco** (reconhecido pelo nome ou escolhido); cartão ligado à conta dele; resumo de saldo, faturas do mês, dívida total e limite disponível; no cartão, **fatura do mês** separada da **dívida total**, uso do limite, melhor dia de compra e **histórico de faturas** recolhido |
| Orçamentos | Limite mensal por categoria, progresso, "copiar do mês anterior" e **✨ Sugerir orçamentos**: valor por categoria a partir dos seus gastos (mediana de 6 meses, sem emergências e picos), cabendo na renda menos parcelas e a meta de poupança, e mais conservador quando os cartões estão com muito limite em uso — revisado e ajustado antes de aplicar |
| Metas | Valor alvo, data, aporte mensal sugerido, metas que acompanham o saldo de uma conta ou de um **cofrinho** (Reserva do Mercado Pago) dessa conta; prioridade entre metas e assistente de quanto guardar por mês a partir da sobra prevista |
| Contas a pagar | Vencimentos únicos, mensais ou anuais (com data limite opcional: encerra sozinha depois do último pagamento), com lembrete no painel; sugere cadastrar os pagamentos que se repetem todo mês no extrato (boleto, débito automático, Pix) |
| Investimentos | Carteira de renda variável (FIIs, ações, ETFs, BDRs) com preço médio, cotação atual, valorização de cada compra, proventos e DY; comparação com a Selic (carteira, cada ativo e cada investimento do Open Finance); distribuição por classe |
| Relatórios | Fluxo de caixa mensal com saldo acumulado, despesas/receitas por categoria, maiores despesas |
| Categorias e regras | Categorias com subcategorias, regras automáticas ("descrição contém UBER → Transporte"), mapeamento das categorias do banco |
| Emergências | Marque despesas como emergência (na transação ou em lote). Orçamentos e relatórios mostram em laranja quando o limite ou o mês só estourou por causa delas |
| Sugestões de categoria | Em Transações › Sugestões de categoria: agrupa transações parecidas, sugere a categoria pelo seu histórico e por palavras-chave e cria regras com um clique |
| Calculadora | Botão flutuante no canto superior direito (atalho Alt+C) |
| Manual | Página **Manual** no menu: passo a passo de cada parte do site (14 capítulos: primeiros passos, conectar bancos, contas e cartões, transações, categorização, regras, categorias, orçamentos, metas, contas a pagar, relatórios, investimentos, configurações, privacidade e backup), com link que abre a tela certa, busca e o botão **Ajuda** em cada tela |
| Erros do sistema | Em Configurações › Erros: avisos e erros da API, da sincronização, das cotações e do câmbio e falhas da tela no navegador, com filtros por período, origem e nível, detalhe (pilha) e "resolvido". Repetições viram uma linha só; os antigos são apagados sozinhos (prazo em Parametrização) |

## Instalação

| Você quer… | Use |
|---|---|
| Só usar, no Windows | o [instalador `.msi`](#windows-instalador-msi) |
| Usar no Linux, ou mexer no código | a [instalação a partir do código](#instalação-a-partir-do-código) |

### Windows: instalador `.msi`

1. **Baixe o instalador.** Na [página de versões](https://github.com/LucasDesenvGit/financas-local/releases/latest), baixe o `FinancasLocal-<versão>.msi`. Se quiser conferir o arquivo, baixe também o `SHA256SUMS.txt`.
2. **Abra o `.msi`.** O instalador ainda não é assinado, então o Windows pode mostrar **"O Windows protegeu o computador"**: clique em **Mais informações** e depois em **Executar assim mesmo**.
   - Para conferir que é o arquivo publicado, rode `Get-FileHash .\FinancasLocal-<versão>.msi` no PowerShell. O resultado tem de ser igual ao número do `SHA256SUMS.txt`.
3. **Instale.** Clique em **Avançar** e **Instalar**, e diga **Sim** quando o Windows pedir permissão de administrador. Não precisa de .NET, Node, Git nem terminal: o site já vem pronto.
4. **Termine.** Deixe marcado **"Abrir o Gerenciador do Finanças Local"** e clique em **Concluir**. O gerenciador mostra **"O site está no ar"**: clique em **Abrir o site** (ou abra **https://127.0.0.1:5443**).
5. **Primeiro acesso.** Crie o usuário e a senha (mínimo de 8 caracteres) e siga o capítulo **Primeiros passos** do **Manual**, no menu do site. Para trazer os dados do banco, veja [Conectar o Open Finance](#conectar-o-open-finance-meu-pluggy).

No Menu Iniciar ficam dois atalhos: **Finanças Local**, que abre o site, e **Gerenciador do Finanças Local**. O gerenciador também ganha um atalho na área de trabalho. Para instalar sem ele, use `msiexec /i FinancasLocal-<versão>.msi ATALHOAREADETRABALHO=0`.

**O que fica onde**

| O quê | Pasta |
|---|---|
| Programa (site e gerenciador) | `C:\Program Files\FinancasLocal` |
| Seus dados: banco, chaves, backups e certificado | `C:\ProgramData\FinancasLocal\dados` (só o sistema e os administradores acessam) |

O site roda como serviço do Windows (**FinancasLocal**). Ele liga sozinho com o computador e só responde nele mesmo, em `https://127.0.0.1:5443`.

**O Gerenciador do Finanças Local**
- **Situação do site:** "no ar" (com a versão), "desligado" ou "ligado, mas não responde".
- **Abrir o site**, **Ligar/Desligar** e **Reiniciar**. As duas últimas pedem permissão de administrador.
- **Versão nova:** procura no GitHub, mostra **O que mudou** e oferece **Atualizar para x.y.z**.
- **Mais opções:**
  - salvar uma cópia do banco onde você escolher (pendrive, nuvem…);
  - ver os erros do site;
  - abrir o manual;
  - **iniciar com o Windows**: fica na área de notificação e avisa se o site cair.

## Atualizar o site

Nenhuma atualização apaga dados: transações, regras, metas e a conexão com o Open Finance ficam. Antes de mudar a estrutura do banco, a versão nova guarda uma cópia em `dados\backups` (`FinancasLocal-antes-atualizacao-….db`, com as 5 mais recentes).

**Que versão eu tenho?** O gerenciador mostra ao lado de "O site está no ar". No site, fica em **Configurações › Parametrização › Sistema**.

### Instalado pelo `.msi`

1. Abra o **Gerenciador do Finanças Local** e clique em **Procurar versão nova**. Ele também procura sozinho ao abrir.
2. Quando aparecer **"Tem versão nova: x.y.z"**, veja **O que mudou** e clique em **Atualizar para x.y.z**.
3. O gerenciador baixa o instalador, confere o SHA256 e abre o instalador. Clique em **Avançar** e **Instalar**, e diga **Sim** quando o Windows pedir.
4. O site fica fora do ar por alguns segundos. No fim, o gerenciador abre de novo, já com a versão nova.

**Sem o gerenciador, o resultado é o mesmo:** baixe o `.msi` novo da [página de versões](https://github.com/LucasDesenvGit/financas-local/releases/latest) e abra. Ele troca a versão instalada e mantém os dados. Para voltar a uma versão anterior, desinstale a atual antes (os dados ficam).

### Instalado pelos scripts e quer passar para o instalador

Vale para quem instalou com o `instalar-windows.cmd` ou o `3-publicar-atualizar.sh`. Baixe o `.msi` e abra. Ele encontra a instalação antiga e:
- **para o site antigo** e **copia o banco e as chaves** para `C:\ProgramData\FinancasLocal\dados`. A pasta antiga **não é apagada**: uma nota com os caminhos fica em `dados\migrado-da-instalacao-por-scripts.txt`;
- **troca o serviço antigo** pelo do instalador e põe o certificado HTTPS novo;
- **se não achar o banco antigo, para sem mudar nada**, diz onde procurou, e o site antigo volta a rodar.

O Client Secret da Pluggy continua funcionando, porque as chaves vêm junto. Depois de conferir no site que está tudo lá, a pasta antiga pode ser apagada.

### Continuar pelos scripts (Linux, ou quem compila do código)

```bash
git pull
scripts/3-publicar-atualizar.sh     # no Windows, num Git Bash aberto como Administrador
```

No Windows sem bash, rode o `instalar-windows.cmd` de novo. Ele atualiza e, se a versão nova não subir, volta para a anterior sozinho. Em qualquer caso, a publicação para o serviço por alguns segundos, copia os arquivos novos e inicia o serviço de novo.

## Desinstalar

- **Instalado pelo `.msi`:** em **Configurações do Windows › Aplicativos › Aplicativos instalados › Finanças Local › Desinstalar**. Saem o programa, o serviço e o certificado; **os dados ficam** em `C:\ProgramData\FinancasLocal\dados`. Para apagar os dados também: `msiexec /x FinancasLocal-<versão>.msi APAGARDADOS=1`.
- **Instalado pelos scripts:** pare com `parar-windows.cmd -Desativar` (Windows) ou `scripts/parar.sh --desativar` (Linux) e apague a pasta do projeto. Os dados ficam em `dados/` e as cópias em `backups/`, dentro dela. No Windows, remova também o serviço, num terminal aberto como Administrador: `sc.exe delete FinancasLocal`.

## Instalação a partir do código

**Windows, a partir do código, com dois cliques:** dê dois cliques em `instalar-windows.cmd`, na raiz do projeto. Ele pede permissão de Administrador e roda `scripts/instalar-windows.ps1`, que faz as três etapas abaixo usando só o PowerShell que já vem no Windows — não precisa instalar Git, bash, Python nem nada antes. No fim, abre o site no navegador. Pode rodar de novo para atualizar: antes, o banco é copiado para `backups/`, e se a versão nova não subir a anterior volta sozinha. Se algo falhar, a janela fica aberta com o erro e a etapa em que parou, e a saída completa fica em `instaladores/instalacao-windows.log`.

**Linux (ou Windows pelo Git Bash), com um comando:**

```bash
cd financas-local/scripts
./instalar.sh                # roda 1, 2 e 3 em sequência e para no primeiro erro
```

**Etapa por etapa (bash).** Abra um terminal com bash disponível (no Linux, o terminal do próprio sistema; no Windows, o **Git Bash**, que vem com o [Git for Windows](https://git-scm.com/download/win), ou o WSL). Abra o Git Bash **como Administrador** para o `3-publicar-atualizar.sh`, que registra o serviço do Windows. O backup (`4-backup.sh`) continua precisando de bash no Windows.

```bash
cd financas-local/scripts
./1-preparar-ambiente.sh     # .NET 10 SDK, Node.js e sqlite3 (poucos minutos)
./2-criar-banco.sh           # cria o banco SQLite com a estrutura completa
./3-publicar-atualizar.sh    # compila e publica como serviço do sistema
```

Depois, abra **https://localhost:5443**. No primeiro acesso você cria o usuário e a senha (mínimo de 8 caracteres).

**Gerenciador (opcional, Windows).** Para ter o Gerenciador do Finanças Local também na instalação pelos scripts, rode, sem precisar de administrador:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\instalar-gerenciador.ps1
```

Ele publica o gerenciador em `publicado\gerenciador` e cria os atalhos no Menu Iniciar e na área de trabalho (`-SemAtalhoNaAreaDeTrabalho` para não criar este último). O gerenciador acha o site pelo serviço e mostra a situação e a versão, liga, desliga e copia o banco. Ele não oferece o `.msi`, porque nessa instalação a versão nova vem pelos scripts. Rode o mesmo comando de novo para atualizar o gerenciador.

### De onde vem cada coisa

| Componente | Origem | Verificação |
|---|---|---|
| .NET 10 SDK (portátil em `ferramentas/dotnet`) | metadados oficiais `builds.dotnet.microsoft.com` | SHA-512 + assinatura do instalador |
| Node.js LTS (portátil em `ferramentas/node`, usado só para compilar o Angular) | `nodejs.org` | SHA-256 do `SHASUMS256.txt` oficial |
| `sqlite3` (CLI) | o do sistema, se houver; senão portátil em `ferramentas/sqlite`, baixado de `www.sqlite.org` (Linux arm64: gerenciador de pacotes) | SHA3-256 publicado na página oficial de download |

`1-preparar-ambiente.sh` confere antes as ferramentas básicas do terminal (`curl`, `tar`, `sha256sum`…, todas presentes no Git Bash), depois verifica cada um desses componentes, só baixa/instala o que ainda não estiver disponível e, no final, mostra a versão de cada um.

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
scripts/3-publicar-atualizar.sh
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
3. **Lançamento manual** em **+ Lançar compra/venda**. Num ativo que já tem histórico da corretora, o seu lançamento só completa o que faltou (ex.: uma compra que a corretora não mandou), e as próximas compras continuam chegando. Se a corretora mandar depois a mesma operação, as duas são ligadas, sem duplicar. Um ativo só com lançamentos seus fica fora da importação.
4. **Conferir investimentos:** compara a carteira com a posição da corretora e com o extrato e sugere as correções (FII cadastrado como ação, cotas que faltam, troco das compras contando como rendimento), que você confirma.

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
- **Erros do sistema** gravados no banco sem dados sensíveis: query strings, tokens, senhas, CPF e valores em R$ são trocados por `[removido]` antes de gravar ([ADR 0022](docs/adr/0022-erros-do-sistema-gravados-no-banco.md)).
- Consultas ao banco somente via LINQ/EF Core, sem SQL montado a partir de texto do usuário.
- **Dados de pagamento do Open Finance:** do Pix ou da TED, só o meio (PIX, TED…) e se quem pagou e quem recebeu são a mesma pessoa são guardados; nomes e CPF/CNPJ de terceiros não são gravados ([ADR 0025](docs/adr/0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md)).
- **Saídas para a internet** (todas HTTPS): `api.pluggy.ai` (Open Finance), `api.bcb.gov.br` (taxa Selic, só datas são enviadas), `olinda.bcb.gov.br` (cotação PTAX de moedas estrangeiras, só o código da moeda e datas são enviados) e `query1.finance.yahoo.com` / `brapi.dev` (cotações, só o código do ativo é enviado). Quantidades, valores e dados pessoais não saem do computador. O token opcional da brapi fica criptografado como o secret da Pluggy. O **Gerenciador do Finanças Local** (Windows, instalado pelo `.msi`) consulta `api.github.com` e baixa do GitHub (`github.com` e o domínio de arquivos do GitHub) a versão nova, conferindo o SHA256 antes de abrir o instalador. Ele não envia nada além do pedido HTTP, e a API não acessa o GitHub ([ADR 0028](docs/adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md)).

Recomendações:
- Ative criptografia de disco (**BitLocker** no Windows, **LUKS**/dm-crypt no Linux), porque o SQLite não criptografa o arquivo do banco.
- Agende execuções periódicas de `scripts/4-backup.sh` e copie de vez em quando a pasta `backups` para um disco externo.
- Rode o `1-preparar-ambiente.sh` a cada poucos meses para atualizar o .NET com as correções de segurança.

## Parar tudo

O sistema instalado é **um processo só**: a API, rodando como serviço do sistema. O front-end (Angular) é publicado dentro dela e roda no navegador, e o banco (SQLite) é só o arquivo `dados/sql/FinancasLocal.db`, aberto pela própria API, sem serviço nem porta.

- **Instalado pelo `.msi`:** no gerenciador, clique em **Desligar** (ou em Serviços do Windows › FinancasLocal › Parar). Para ligar de novo, use **Ligar**.
- **Windows, instalado pelos scripts:** dê dois cliques em `parar-windows.cmd`, na raiz (pede permissão de Administrador; não precisa de bash).
- **Linux/macOS:** `scripts/parar.sh`.

Os dois param o serviço, a API rodando fora dele (`dotnet run`, Visual Studio) e o front de desenvolvimento (`npm start`), e avisam se algum programa ainda está com o banco aberto. Opções: `--desativar` / `-Desativar` (não sobe sozinho no próximo login/boot) e `--compiladores` / `-Compiladores` (encerra também os servidores de build do .NET). Para subir de novo, rode a publicação (`3-publicar-atualizar.sh` ou `instalar-windows.cmd`).

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
├── instalador/            instalador .msi do Windows (WiX): FinancasLocal.wxs, configurar.ps1 e gerar-msi.ps1
├── scripts/               instalação, publicação, backup e parada (bash; no Windows também instalar-windows.ps1 e parar-windows.ps1)
│   └── git/              configuração do repositório no GitHub (PowerShell)
├── src/backend/           Financas.Api (C# / .NET 10 / EF Core) e Financas.MigrarSqlite (migração do SQL Server)
├── src/frontend/          financas-web (Angular)
├── src/gerenciador/       Financas.Gerenciador (app do Windows instalado pelo .msi)
├── dados/                 [gerado] banco (SQLite), chaves, logs — acesso restrito
├── backups/               [gerado] backups do banco
├── publicado/             [gerado] aplicação publicada
├── ferramentas/           [gerado] .NET SDK, Node.js, sqlite3 (se preciso), caches NuGet/npm
└── instaladores/          [gerado] instaladores baixados (podem ser apagados)
```

## Problemas comuns

| Sintoma | O que fazer |
|---|---|
| Windows: `Python was not found`, `sqlite3: command not found` ou `unzip: command not found` | Versão antiga dos scripts. Atualize o repositório e use o `instalar-windows.cmd`, que não depende de bash nem de Python |
| Windows: o `instalar-windows.cmd` falhou | Veja a linha `ERRO` na janela e o log completo em `instaladores/instalacao-windows.log` |
| Windows: `$'\r': command not found` | Os `.sh` foram baixados com quebra de linha do Windows (CRLF). Clone o repositório com o Git (o `.gitattributes` força LF) em vez de baixar o zip, ou rode `git rm --cached -r . && git reset --hard` |
| Windows: o serviço não é registrado/iniciado | Rode o `3-publicar-atualizar.sh` num Git Bash aberto como Administrador |
| "O Windows protegeu o computador" ao abrir o `.msi` | O instalador ainda não é assinado. Clique em **Mais informações › Executar assim mesmo**. Para conferir o arquivo, compare o `Get-FileHash` dele com o `SHA256SUMS.txt` do mesmo Release |
| A instalação pelo `.msi` parou dizendo que o banco da instalação antiga não foi encontrado | Nada foi mudado e o site antigo voltou a rodar. Confira o `Banco:Caminho` no `appsettings.Production.json` da instalação antiga (pasta `publicado\api`). Outra saída: faça uma cópia do banco e remova o serviço antigo antes de instalar |
| O gerenciador não oferece a versão nova | Ele só oferece quando o Release tem o `.msi` e o `SHA256SUMS.txt`. Sem internet, ele avisa e o site continua funcionando. Você também pode baixar o `.msi` novo e abrir |
| O gerenciador diz "ligado, mas não responde" | Espere alguns segundos (o site está subindo) e clique em **Reiniciar**. Se continuar, veja o Visualizador de Eventos do Windows › Aplicativo, origem FinancasLocal |
| Navegador alerta sobre o certificado | Instalado pelo `.msi`: abra o mesmo `.msi` de novo e escolha **Reparar**. Instalado pelos scripts: rode o `3-publicar-atualizar.sh` de novo. Os dois recolocam o certificado nos confiáveis. No Firefox, em `about:config`, ative `security.enterprise_roots.enabled` |
| Algo deu errado e não sei o quê | Veja Configurações › Erros: a mensagem, quando e quantas vezes aconteceu e, em "detalhe", a pilha do erro |
| Sincronização falha com 401/403 | Confira o Client ID e o Secret e se o MeuPluggy foi autorizado para aquele banco |
| O secret da Pluggy não abre mais (depois de migrar com o `Financas.MigrarSqlite`, trocar de máquina/SO ou apagar `dados\chaves`) | As chaves de proteção de dados são específicas da instalação: informe o Client Secret de novo em Configurações |
