# Segurança

O resumo está no [README](../README.md#segurança). Aqui fica o detalhe: o que sai para a internet, as recomendações e como o site se protege. De quem ele se protege e o que fica de fora: [modelo de ameaças](MODELO-DE-AMEACAS.md). Achou uma falha? Veja o [SECURITY.md](../SECURITY.md).

**Seus dados ficam no seu computador**, na pasta `dados`: em `C:\ProgramData\FinancasLocal\dados` na instalação pelo `.msi` (só o sistema e os administradores acessam), ou na pasta `dados` do projeto na instalação pelos scripts (no Linux, só a sua conta). O site só responde no próprio computador, por HTTPS, e pede usuário e senha. A exceção é o **acesso pelo celular** (Configurações › Segurança), desligado por padrão: ligado, o site responde também para os aparelhos da sua rede de casa, e a senha passa a ser a proteção contra quem estiver nela ([ADR 0061](adr/0061-acesso-pelo-celular-na-rede-local.md)).

## O que sai para a internet

Sempre por HTTPS, com o mínimo de dados; quantidades, valores e dados pessoais não saem.

| Para onde | Para quê | O que vai |
|---|---|---|
| `api.pluggy.ai` | Open Finance (extratos, saldos, investimentos) | as credenciais da Pluggy que você informou |
| `api.bcb.gov.br` | taxa Selic | só datas |
| `olinda.bcb.gov.br` | cotação PTAX de moedas estrangeiras | só o código da moeda e datas |
| `query1.finance.yahoo.com` / `brapi.dev` | cotações de ações e FIIs (fontes gratuitas e não oficiais) | o código do ativo; para a `brapi.dev`, também o seu token (no cabeçalho `Authorization`, fora da URL), se você informou um |
| `g1.globo.com` / `feeds.bbci.co.uk` / `www.tabnews.com.br` | só com o módulo **Notícias** ligado (vem desligado): as notícias do dia, a cada 45 minutos ([ADR 0067](adr/0067-noticias-g1-bbc-e-tabnews-so-com-o-modulo-ligado.md)) | só o pedido do feed, sem nada seu |
| `api.mercadobitcoin.net` | cotação de criptomoedas em reais, a cada 30 minutos ([ADR 0048](adr/0048-cotacao-de-criptomoedas-pelo-mercado-bitcoin.md)) | só os pares fixos (BTC-BRL, ETH-BRL, SOL-BRL, USDT-BRL) |
| `api.github.com` / `github.com` | só o **Gerenciador** do Windows, para achar e baixar a versão nova, conferindo o SHA256 ([ADR 0028](adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md)) | só o pedido da página de versões. O gerenciador do Linux não acessa a internet (só na instalação de desenvolvimento, pelo `git`, quando se clica em "Atualizar para a última do develop"; [ADR 0040](adr/0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md)) |

## Recomendações

- Ative a criptografia do disco (**BitLocker** no Windows, **LUKS** no Linux): o arquivo do banco não é criptografado.
- De vez em quando, salve uma cópia do banco pelo gerenciador (**Mais opções**) num pendrive ou disco externo.
- Vai ligar o **acesso pelo celular**? Use uma senha forte (quem está no seu Wi-Fi chega à tela de entrar), ligue só na rede de casa e reserve o IP do computador no roteador.

## Detalhes técnicos

- **Sem exposição na rede, a não ser pelo acesso pelo celular.** O próprio executável do site (em Go, sem IIS nem proxy reverso) responde em `127.0.0.1`, via HTTPS: um endereço do site que não seja de loopback é recusado ao iniciar (`config.ConferirSoLocal`). O SQLite é só um arquivo local: não tem porta nem listener de rede.
- **Acesso pelo celular** ([ADR 0061](adr/0061-acesso-pelo-celular-na-rede-local.md), `internal/plataforma/celular`), desligado por padrão e ligado só com login, em Configurações › Segurança:
  - abre um segundo servidor só no IP escolhido, que tem de ser um IPv4 privado (RFC 1918) deste computador, com o mesmo login, CSRF e cabeçalhos, e só esse IP nos hosts permitidos;
  - o certificado é de uma autoridade local (`dados/chaves/celular-autoridade.*`), com restrição de nomes: só assina IPs de rede de casa, nunca um nome de site. O certificado do computador não muda;
  - no Windows, a regra do firewall vale só no perfil Particular e só para a sub-rede local (`remoteip=localsubnet`); no Linux o site não mexe no firewall e mostra o comando;
  - a única rota sem login é a da autoridade (`GET /api/celular/autoridade`, só o certificado público, só com o acesso ligado);
  - ligar e desligar entram na auditoria, com o IP de quem fez.
- **Um processo só** ([ADR 0049](adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)): até a #86, o Go rodava atrás de uma API em .NET, numa porta interna protegida por um segredo; hoje o site é um executável só, sem porta interna.
- **Sem senhas em arquivos.** O SQLite não tem login de banco: o acesso é a permissão do sistema operacional sobre a pasta `dados`, que só a conta do serviço (Serviço do Windows ou systemd) e os administradores leem.
- **Client Secret da Pluggy e token da brapi criptografados** no banco com AES-256-GCM (`internal/plataforma/segredos`, prefixo `v2:`). A chave fica em `dados\chaves\segredos.chave`: no Windows, protegida pelo DPAPI; no Linux, pela permissão do arquivo (0600). A API nunca devolve o secret.
- **Login:** senha com hash PBKDF2-SHA512 (no formato do ASP.NET Identity, para as senhas de antes continuarem valendo), bloqueio depois de errar a senha (padrão: 15 min após 5 erros, ajustável em **Configurações › Parametrização**, de 3 a 20 tentativas e de 1 a 1.440 minutos), limite de 5 tentativas por minuto por IP (cada aparelho da rede tem o seu) e tempo de resposta igual para usuário inexistente.
- **Sessão:** cookie `__Host-` HttpOnly + Secure + SameSite=Strict, que expira em 2 h sem uso. Trocar a senha derruba as outras sessões.
- **CSRF:** token antifalsificação validado em todo POST, PUT, PATCH e DELETE.
- **Cabeçalhos:** CSP restritiva (sem scripts de terceiros nem script inline; nada é carregado da internet), X-Frame-Options, nosniff, Referrer-Policy e `no-store` nas respostas da API.
- **Validação de host:** só `localhost`/`127.0.0.1` (e, no servidor do acesso pelo celular, só o IP escolhido), contra DNS rebinding.
- **Auditoria** em Configurações › Segurança: logins, falhas, bloqueios e saídas, trocas de senha, parâmetros, credenciais da Pluggy, conexões adicionadas e removidas, alterações em lote nas transações (categoria, renomear, regras de descrição), conferências de investimentos e trocos automáticos, o acesso pelo celular ligado e desligado, e o desfazer de cada um.
- **Erros do sistema** gravados sem dados sensíveis: query strings, tokens, senhas, CPF e valores em R$ viram `[removido]` antes de gravar ([ADR 0022](adr/0022-erros-do-sistema-gravados-no-banco.md)).
- Consultas ao banco somente parametrizadas (`sqlc` ou `?`), sem SQL montado a partir de texto do usuário.
- **Dados de pagamento do Open Finance:** do Pix ou da TED, o nome e o CPF/CNPJ do outro lado não são gravados em campo próprio: fica o meio e se quem pagou e quem recebeu são a mesma pessoa ([ADR 0025](adr/0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md)). A descrição, porém, é guardada como o banco manda e costuma trazer o nome ("PIX ENVIADO FULANO"); veja [Dados pessoais](DADOS-PESSOAIS.md#dados-bancários-da-pluggy).
