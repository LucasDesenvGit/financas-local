# Segurança

O resumo está no [README](../README.md#segurança). Aqui fica o detalhe: o que sai para a internet, as recomendações e como o site se protege. De quem ele se protege e o que fica de fora: [modelo de ameaças](MODELO-DE-AMEACAS.md). Achou uma falha? Veja o [SECURITY.md](../SECURITY.md).

**Seus dados ficam no seu computador**, na pasta `dados`: em `C:\ProgramData\FinancasLocal\dados` na instalação pelo `.msi` (só o sistema e os administradores acessam), ou na pasta `dados` do projeto na instalação pelos scripts (no Linux, só a sua conta). O site só responde no próprio computador, por HTTPS, e pede usuário e senha.

## O que sai para a internet

Sempre por HTTPS, com o mínimo de dados; quantidades, valores e dados pessoais não saem.

| Para onde | Para quê | O que vai |
|---|---|---|
| `api.pluggy.ai` | Open Finance (extratos, saldos, investimentos) | as credenciais da Pluggy que você informou |
| `api.bcb.gov.br` | taxa Selic | só datas |
| `olinda.bcb.gov.br` | cotação PTAX de moedas estrangeiras | só o código da moeda e datas |
| `query1.finance.yahoo.com` / `brapi.dev` | cotações de ações e FIIs (fontes gratuitas e não oficiais) | o código do ativo; para a `brapi.dev`, também o seu token (no cabeçalho `Authorization`, fora da URL), se você informou um |
| `api.mercadobitcoin.net` | cotação de criptomoedas em reais, a cada 30 minutos ([ADR 0048](adr/0048-cotacao-de-criptomoedas-pelo-mercado-bitcoin.md)) | só os pares fixos (BTC-BRL, ETH-BRL, SOL-BRL, USDT-BRL) |
| `api.github.com` / `github.com` | só o **Gerenciador** do Windows, para achar e baixar a versão nova, conferindo o SHA256 ([ADR 0028](adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md)) | só o pedido da página de versões. O gerenciador do Linux não acessa a internet (só na instalação de desenvolvimento, pelo `git`, quando se clica em "Atualizar para a última do develop"; [ADR 0040](adr/0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md)) |

## Recomendações

- Ative a criptografia do disco (**BitLocker** no Windows, **LUKS** no Linux): o arquivo do banco não é criptografado.
- De vez em quando, salve uma cópia do banco pelo gerenciador (**Mais opções**) num pendrive ou disco externo.

## Detalhes técnicos

- **Sem exposição na rede.** O Kestrel (servidor web embutido na própria API, sem IIS nem proxy reverso) responde somente em `127.0.0.1`, via HTTPS. O SQLite é só um arquivo local: não tem porta nem listener de rede.
- **Back-end em Go atrás da API** ([ADR 0049](adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)): a API inicia o Go como processo filho e repassa a ele só as rotas já migradas, depois de conferir o login e o CSRF. O Go escuta só em `127.0.0.1`, numa porta escolhida na partida, e recusa (401) qualquer pedido sem o segredo interno, gerado a cada partida e conhecido só pela API. Ele para junto com a API.
- **Sem senhas em arquivos.** O SQLite não tem login de banco: o acesso é a permissão do sistema operacional sobre a pasta `dados`, que só a conta do serviço (Serviço do Windows ou systemd) e os administradores leem.
- **Client Secret da Pluggy e token da brapi criptografados** no banco com ASP.NET Data Protection. As chaves ficam em `dados\chaves`: no Windows, protegidas por DPAPI; no Linux, pelas permissões da pasta. A API nunca devolve o secret.
- **Login:** senha com hash PBKDF2-SHA512 (ASP.NET Identity), bloqueio depois de errar a senha (padrão: 15 min após 5 erros, ajustável em **Configurações › Parametrização**, de 3 a 20 tentativas e de 1 a 1.440 minutos), limite de 5 tentativas por minuto e tempo de resposta igual para usuário inexistente.
- **Sessão:** cookie `__Host-` HttpOnly + Secure + SameSite=Strict, que expira em 2 h sem uso. Trocar a senha derruba as outras sessões.
- **CSRF:** token antifalsificação validado em todo POST, PUT, PATCH e DELETE.
- **Cabeçalhos:** CSP restritiva (sem scripts de terceiros nem script inline; nada é carregado da internet), X-Frame-Options, nosniff, Referrer-Policy e `no-store` nas respostas da API.
- **Validação de host:** só `localhost`/`127.0.0.1`, contra DNS rebinding.
- **Auditoria** em Configurações › Segurança: logins, falhas, bloqueios e saídas, trocas de senha, parâmetros, credenciais da Pluggy, conexões adicionadas e removidas, alterações em lote nas transações (categoria, renomear, regras de descrição), conferências de investimentos e trocos automáticos, e o desfazer de cada um.
- **Erros do sistema** gravados sem dados sensíveis: query strings, tokens, senhas, CPF e valores em R$ viram `[removido]` antes de gravar ([ADR 0022](adr/0022-erros-do-sistema-gravados-no-banco.md)).
- Consultas ao banco somente via LINQ/EF Core, sem SQL montado a partir de texto do usuário.
- **Dados de pagamento do Open Finance:** do Pix ou da TED, o nome e o CPF/CNPJ do outro lado não são gravados em campo próprio: fica o meio e se quem pagou e quem recebeu são a mesma pessoa ([ADR 0025](adr/0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md)). A descrição, porém, é guardada como o banco manda e costuma trazer o nome ("PIX ENVIADO FULANO"); veja [Dados pessoais](DADOS-PESSOAIS.md#dados-bancários-da-pluggy).
