# Finanças Local

Seu controle financeiro pessoal, rodando **no seu computador**: transações, orçamentos, metas, contas a pagar, investimentos e relatórios. Os extratos dos bancos chegam sozinhos pelo **Open Finance** (via Meu Pluggy), e os seus dados **não saem do computador**.

- **Windows:** instale pelo [instalador `.msi`](#instalar-no-windows) — não precisa de terminal nem de programa nenhum antes.
- **Linux, ou para mexer no código:** veja [Instalar a partir do código](docs/INSTALACAO-PELO-CODIGO.md).
- **Já instalou?** O passo a passo de cada tela está no **Manual**, no menu do próprio site, e o botão **Ajuda** de cada tela abre a parte certa.

## Instalar no Windows

1. **Baixe** o `FinancasLocal-<versão>.msi` da [versão mais recente](https://github.com/LucasDesenvGit/financas-local/releases/latest).
2. **Abra o `.msi`.** O instalador ainda não é assinado, então o Windows pode mostrar **"O Windows protegeu o computador"**: clique em **Mais informações** e depois em **Executar assim mesmo**.
3. **Instale:** **Avançar**, **Instalar** e **Sim** quando o Windows pedir permissão de administrador.
4. **Termine** com **"Abrir o Gerenciador do Finanças Local"** marcado. Quando ele mostrar **"O site está no ar"**, clique em **Abrir o site** (ou abra **https://127.0.0.1:5443**).
5. **Primeiro acesso:** crie o seu usuário e senha (mínimo de 8 caracteres) e [conecte seus bancos](#conectar-seus-bancos-open-finance).

O site liga sozinho com o computador e só responde nele mesmo. No Menu Iniciar ficam os atalhos **Finanças Local** (o site) e **Gerenciador do Finanças Local**, que também fica na área de trabalho.

<details>
<summary>Conferir o arquivo baixado, instalar sem o atalho, versão de desenvolvimento</summary>

- **Conferir o arquivo:** baixe também o `SHA256SUMS.txt` e rode `Get-FileHash .\FinancasLocal-<versão>.msi` no PowerShell. O resultado tem de ser igual ao número do arquivo.
- **Sem o atalho na área de trabalho:** `msiexec /i FinancasLocal-<versão>.msi ATALHOAREADETRABALHO=0`.
- **Versão de desenvolvimento:** quer testar o que já foi aprovado e ainda não saiu? Baixe o `FinancasLocal-desenvolvimento.msi` do [pré-lançamento "desenvolvimento"](https://github.com/LucasDesenvGit/financas-local/releases/tag/desenvolvimento). Ele é gerado a cada mudança aprovada, instala e atualiza igual e mantém os dados. O site mostra a versão como `X.Y.Z-dev.N`, e o gerenciador dessa instalação avisa e atualiza para a versão de desenvolvimento mais nova (quem instalou uma versão publicada continua recebendo só as publicadas).

</details>

### O Gerenciador do Finanças Local

- Mostra se o site está **no ar** (com a versão), **desligado** ou **ligado, mas sem responder**.
- **Abrir o site**, **Ligar/Desligar** e **Reiniciar** (as duas últimas pedem permissão de administrador).
- **Versão nova:** avisa, mostra **O que mudou** e atualiza com um clique.
- **Mais opções:** salvar uma cópia do banco onde você escolher (pendrive, nuvem…), ver os erros do site, abrir o manual e **iniciar com o Windows** (fica na área de notificação e avisa se o site cair).
- **Botão direito no ícone da área de notificação:** mostra a situação do site ("Finanças Local · No ar") e tem **Abrir o site**, **Mostrar o gerenciador** e **Sair**, nas cores do site.

## Conectar seus bancos (Open Finance)

No Brasil, só instituições autorizadas pelo Banco Central acessam o Open Finance direto. Por isso o site usa o **Meu Pluggy**, gratuito para os dados do seu próprio CPF.

1. Em **meu.pluggy.ai**, crie a conta e conecte seus bancos. O consentimento é dado no app de cada banco.
2. Em **dashboard.pluggy.ai**, crie uma conta de desenvolvedor, adicione o conector *MeuPluggy* e crie uma aplicação de desenvolvimento. Copie o **Client ID** e o **Client Secret**.
3. Ainda no dashboard, na aplicação de demonstração, conecte o *MeuPluggy* e autorize **uma vez por banco**. Cada autorização gera um **Item ID**.
4. No site, em **Configurações › Open Finance**, informe o Client ID e o Secret, clique em **Testar conexão** e adicione cada Item ID em **Conexões**.

A primeira carga traz os últimos 365 dias. Depois, o site sincroniza sozinho a cada 6 horas. O consentimento vale até 12 meses, e o Painel avisa 30 dias antes de vencer (a renovação é no Meu Pluggy). O capítulo **Conectar seus bancos** do Manual tem o passo a passo com mais detalhes.

## O que tem

| Tela | Para quê |
|---|---|
| **Painel** | Como você está hoje (patrimônio, saldo, dívida dos cartões, próximos vencimentos) e como está o mês (receitas × despesas, faturas, gastos por categoria, avisos) |
| **Transações** | Tudo o que entrou e saiu, com filtros, o tipo de cada lançamento (Pix, compra no cartão, fatura…), categorização em lote, renomear e lançamentos manuais |
| **Contas e cartões** | Saldo de cada conta, fatura do mês × dívida total e histórico de faturas de cada cartão |
| **Orçamentos** | Limite por categoria e **✨ Sugerir orçamentos** pelos seus gastos |
| **Metas** | Quanto guardar por mês em cada meta, pela sobra prevista |
| **Contas a pagar** | Vencimentos com lembrete no Painel e sugestões tiradas do extrato |
| **Investimentos** | Carteira de ações e FIIs (preço médio, proventos, DY, mês a mês), comparação com a Selic, reservas e empréstimos |
| **Relatórios, Visão anual e Projeção** | Para onde foi o dinheiro, como foi o ano e quanto você vai ter |
| **Categorias e regras** | Suas categorias e regras automáticas ("descrição contém UBER → Transporte") |
| **Manual** | O passo a passo de cada tela, com busca e o botão **Ajuda** em cada uma |

E também: botão para **ocultar os valores** da tela, calculadora (Alt+C), tema claro e escuro, gastos de **emergência** marcados à parte e os erros do sistema em **Configurações › Erros**.

## Atualizar

Nenhuma atualização apaga dados. Antes de mudar a estrutura do banco, a versão nova guarda uma cópia em `dados\backups`. A versão instalada aparece no gerenciador e no site, em **Configurações › Parametrização › Sistema**.

- **Pelo gerenciador:** quando aparecer **"Tem versão nova"**, veja **O que mudou** e clique em **Atualizar**. Ele baixa, confere e abre o instalador; é só **Avançar**, **Instalar** e **Sim**. O site fica fora do ar por alguns segundos.
- **Sem o gerenciador:** baixe o `.msi` novo da [versão mais recente](https://github.com/LucasDesenvGit/financas-local/releases/latest) e abra. Ele troca a versão instalada e mantém os dados. Para voltar a uma versão anterior, desinstale a atual antes (os dados ficam).

### Instalado pelos scripts e quer passar para o instalador

Vale para quem instalou com o `instalar-windows.cmd` ou o `3-publicar-atualizar.sh`: baixe o `.msi` e abra. Ele encontra a instalação antiga, **copia o banco e as chaves** para `C:\ProgramData\FinancasLocal\dados` e troca o serviço antigo pelo dele. A pasta antiga **não é apagada**, e se o banco antigo não for achado, ele para sem mudar nada e o site antigo volta a rodar. O Client Secret da Pluggy continua funcionando. Daí em diante, a versão nova chega pelo gerenciador.

Prefere continuar pelos scripts? Veja [Atualizar, na instalação pelo código](docs/INSTALACAO-PELO-CODIGO.md#atualizar).

## Desinstalar

Em **Configurações do Windows › Aplicativos › Aplicativos instalados › Finanças Local › Desinstalar**. Saem o programa, o serviço e o certificado; **os seus dados ficam** em `C:\ProgramData\FinancasLocal\dados`. Para apagar os dados também: `msiexec /x FinancasLocal-<versão>.msi APAGARDADOS=1`.

Para só desligar o site por um tempo, use **Desligar** no gerenciador.

## Segurança

**Seus dados ficam no seu computador**, na pasta `C:\ProgramData\FinancasLocal\dados` (no Windows, só o sistema e os administradores acessam). O site só responde no próprio computador, por HTTPS, e pede usuário e senha.

**O que sai para a internet** (sempre por HTTPS, com o mínimo de dados; quantidades, valores e dados pessoais não saem):

| Para onde | Para quê | O que vai |
|---|---|---|
| `api.pluggy.ai` | Open Finance (extratos, saldos, investimentos) | as credenciais da Pluggy que você informou |
| `api.bcb.gov.br` | taxa Selic | só datas |
| `olinda.bcb.gov.br` | cotação PTAX de moedas estrangeiras | só o código da moeda e datas |
| `query1.finance.yahoo.com` / `brapi.dev` | cotações de ações e FIIs (fontes gratuitas e não oficiais) | só o código do ativo |
| `api.mercadobitcoin.net` | cotação de criptomoedas em reais, a cada 30 minutos (ADR 0048) | só os pares fixos (BTC-BRL, ETH-BRL, SOL-BRL, USDT-BRL) |
| `api.github.com` / `github.com` | só o **Gerenciador** do Windows, para achar e baixar a versão nova, conferindo o SHA256 ([ADR 0028](docs/adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md)) | só o pedido da página de versões. O gerenciador do Linux não acessa a internet (só na instalação de desenvolvimento, pelo `git`, quando se clica em "Atualizar para a última do develop"; [ADR 0040](docs/adr/0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md)) |

**Recomendações:**
- Ative a criptografia do disco (**BitLocker** no Windows, **LUKS** no Linux): o arquivo do banco não é criptografado.
- De vez em quando, salve uma cópia do banco pelo gerenciador (**Mais opções**) num pendrive ou disco externo.

<details>
<summary>Detalhes técnicos de segurança</summary>

- **Sem exposição na rede.** O Kestrel (servidor web embutido na própria API, sem IIS nem proxy reverso) responde somente em `127.0.0.1`, via HTTPS. O SQLite é só um arquivo local: não tem porta nem listener de rede.
- **Back-end em Go atrás da API** ([ADR 0049](docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)): a API inicia o Go como processo filho e repassa a ele só as rotas já migradas, depois de conferir o login e o CSRF. O Go escuta só em `127.0.0.1`, numa porta escolhida na partida, e recusa (401) qualquer pedido sem o segredo interno, gerado a cada partida e conhecido só pela API. Ele para junto com a API.
- **Sem senhas em arquivos.** O SQLite não tem login de banco: o acesso é a permissão do sistema operacional sobre a pasta `dados`, que só a conta do serviço (Serviço do Windows ou systemd) e os administradores leem.
- **Client Secret da Pluggy e token da brapi criptografados** no banco com ASP.NET Data Protection. As chaves ficam em `dados\chaves`: no Windows, protegidas por DPAPI; no Linux, pelas permissões da pasta. A API nunca devolve o secret.
- **Login:** senha com hash PBKDF2-SHA512 (ASP.NET Identity), bloqueio de 15 min após 5 erros, limite de 5 tentativas por minuto e tempo de resposta igual para usuário inexistente.
- **Sessão:** cookie `__Host-` HttpOnly + Secure + SameSite=Strict, que expira em 2 h sem uso. Trocar a senha derruba as outras sessões.
- **CSRF:** token antifalsificação validado em todo POST, PUT, PATCH e DELETE.
- **Cabeçalhos:** CSP restritiva (sem scripts de terceiros nem script inline; nada é carregado da internet), X-Frame-Options, nosniff, Referrer-Policy e `no-store` nas respostas da API.
- **Validação de host:** só `localhost`/`127.0.0.1`, contra DNS rebinding.
- **Auditoria** de logins, falhas, trocas de senha e alterações de credenciais, visível em Configurações.
- **Erros do sistema** gravados sem dados sensíveis: query strings, tokens, senhas, CPF e valores em R$ viram `[removido]` antes de gravar ([ADR 0022](docs/adr/0022-erros-do-sistema-gravados-no-banco.md)).
- Consultas ao banco somente via LINQ/EF Core, sem SQL montado a partir de texto do usuário.
- **Dados de pagamento do Open Finance:** do Pix ou da TED, só o meio e se quem pagou e quem recebeu são a mesma pessoa; nomes e CPF/CNPJ de terceiros não são gravados ([ADR 0025](docs/adr/0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md)).

</details>

## Problemas comuns

| Sintoma | O que fazer |
|---|---|
| "O Windows protegeu o computador" ao abrir o `.msi` | O instalador ainda não é assinado. Clique em **Mais informações › Executar assim mesmo** |
| O gerenciador diz "ligado, mas não responde" | Espere alguns segundos (o site está subindo) e clique em **Reiniciar**. Se continuar, veja o Visualizador de Eventos do Windows › Aplicativo, origem FinancasLocal |
| O gerenciador não oferece a versão nova | Sem internet, ele avisa e o site continua funcionando. Você também pode baixar o `.msi` novo e abrir |
| O navegador alerta sobre o certificado | Abra o mesmo `.msi` de novo e escolha **Reparar**: ele recoloca o certificado nos confiáveis. No Firefox, em `about:config`, ative `security.enterprise_roots.enabled` |
| A instalação falhou e o log diz que a porta 5443 está em uso | Outro programa está na porta do site, e o log do instalador diz qual e o PID. Feche esse programa (ou pare o serviço dele) e abra o `.msi` de novo. Depois de instalado, o gerenciador também avisa quando a porta está ocupada |
| A instalação parou dizendo que o banco da instalação antiga não foi encontrado | Nada foi mudado e o site antigo voltou a rodar. Confira o `Banco:Caminho` no `appsettings.Production.json` da instalação antiga (pasta `publicado\api`), ou faça uma cópia do banco e remova o serviço antigo antes de instalar |
| A sincronização falha com 401/403 | Confira o Client ID e o Secret e se o MeuPluggy foi autorizado para aquele banco |
| O Client Secret da Pluggy não abre mais (trocou de computador ou apagou `dados\chaves`) | As chaves são de cada instalação: informe o Client Secret de novo em Configurações |
| Algo deu errado e não sei o quê | Veja **Configurações › Erros**: a mensagem, quando e quantas vezes aconteceu |

Instalou pelos scripts? Os problemas de lá estão em [Instalar a partir do código](docs/INSTALACAO-PELO-CODIGO.md#problemas-comuns-instalação-pelos-scripts).

## Para quem desenvolve

| Documento | Conteúdo |
|---|---|
| [docs/INSTALACAO-PELO-CODIGO.md](docs/INSTALACAO-PELO-CODIGO.md) | Instalar, atualizar e parar pelos scripts (Linux e Windows), migrar do SQL Server, Visual Studio, testes (Jest e no navegador) e estrutura de pastas |
| [CONTRIBUINDO.md](CONTRIBUINDO.md) | Branches, commits, versão, pull requests e como publicar uma versão |
| [docs/ARQUITETURA.md](docs/ARQUITETURA.md) | Camadas, pastas e decisões técnicas |
| [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md) | Como alterar a estrutura do banco (sem migrations) |
| [docs/adr/](docs/adr/README.md) | Decisões de arquitetura (ADRs): por que cada escolha foi feita |
| [CHANGELOG.md](CHANGELOG.md) | O que mudou em cada versão |

API em C# (.NET 10) hospedada pelo Kestrel, front-end em Angular e banco SQLite, publicados juntos num serviço do sistema.
