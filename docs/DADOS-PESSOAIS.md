# Dados pessoais

O que o Finanças Local guarda, onde, o que sai do computador, por quanto tempo fica e como apagar.

> Este é um documento técnico de transparência, conferido no código. Não é uma política de privacidade jurídica. A LGPD serve de roteiro: o sistema roda no seu computador, e você é quem controla os dados.

## Em resumo

- **Tudo fica no seu computador**, na pasta `dados`. Não há servidor do projeto: ninguém além de você recebe esses dados.
- **O que sai** vai para os serviços da [tabela de saídas](SEGURANCA.md#o-que-sai-para-a-internet), sempre o mínimo. Quantidades, valores e dados pessoais não saem.
- **Para quem o sistema traz dados:** a Pluggy entrega extratos, saldos e investimentos dos bancos que você autorizou no Meu Pluggy.
- **O banco não é criptografado.** Só os segredos são. Por isso, a recomendação é criptografar o disco (BitLocker ou LUKS).

## Por grupo de dados

Todos os grupos ficam no banco (`dados/sql/FinancasLocal.db`) e também em **todas as [cópias](#cópias)**.

### Login e auditoria

| | |
|---|---|
| **O que é** | `Usuario`: login, hash da senha (PBKDF2), tentativas e último login. `AuditoriaAcesso`: evento, login, IP e navegador de cada ação registrada |
| **De onde vem** | De você, e do próprio sistema |
| **Sai do computador?** | Não |
| **Por quanto tempo** | Sem prazo: a auditoria não é apagada sozinha |
| **Como apagar** | Só apagando tudo. A tela de Segurança lista a auditoria, mas não apaga |

### Credenciais

| | |
|---|---|
| **O que é** | `Configuracao`: o Client ID da Pluggy, em texto, e o Client Secret da Pluggy e o token da brapi, criptografados com as chaves de `dados/chaves` |
| **De onde vem** | De você, em **Configurações** |
| **Sai do computador?** | Sim: as credenciais da Pluggy vão para `api.pluggy.ai`, e o token, para `brapi.dev`, por HTTPS |
| **Por quanto tempo** | Até você trocar ou remover |
| **Como apagar** | Remova em **Configurações**. A API nunca devolve o segredo |

### Dados bancários da Pluggy

| | |
|---|---|
| **O que é** | `ConexaoBancaria`: a instituição e o identificador na Pluggy. `Conta`: nome, número como o banco informa, saldo e limite. `FaturaCartao`: valores e datas. `Transacao`: descrição, descrição original, estabelecimento, valor e data. `Investimento`, `HistoricoInvestimento`, `Emprestimo` e `PagamentoEmprestimo`: nome, emissor, saldos e parcelas. `SincronizacaoLog`: quando rodou e a mensagem |
| **De onde vem** | Do Open Finance, pela Pluggy, dos bancos que você autorizou |
| **Sai do computador?** | Não. Esses dados só entram |
| **Por quanto tempo** | Sem prazo, enquanto a conexão existir |
| **Como apagar** | **Remover a conexão** em Configurações apaga, em cascata, as contas, transações, investimentos e empréstimos dela. Uma transação do Open Finance sozinha não pode ser excluída; dá para marcá-la para ignorar nos relatórios |

**Dados de terceiros:** o nome e o CPF/CNPJ do outro lado de um Pix ou TED não são gravados em campo próprio. Do documento, fica só o sim ou não de "mesmo titular" (`MesmaTitularidade`, [ADR 0025](adr/0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md)). **Mas a descrição vem do banco como está**, e ela costuma trazer o nome de quem recebeu ou pagou, como em "PIX ENVIADO FULANO". Esse texto fica em `Descricao` e `DescricaoOriginal`.

### Lançamentos e planejamento feitos por você

| | |
|---|---|
| **O que é** | Contas e transações manuais, `Categoria`, `RegraCategorizacao`, `RegraDescricao`, `Orcamento`, `Meta`, `MetaAporte`, `ContaAgendada` e `ReservaConferencia`. Regras e contas a pagar podem conter nomes que você digitou |
| **De onde vem** | De você |
| **Sai do computador?** | Não |
| **Por quanto tempo** | Até você apagar |
| **Como apagar** | Nas telas de cada um |

### Carteira

| | |
|---|---|
| **O que é** | `Ativo`, `OperacaoAtivo`, `ProventoAtivo` e `PrecoMensalAtivo`: o que você tem, as quantidades e os preços |
| **De onde vem** | De você, do extrato e do Open Finance. As cotações vêm de Yahoo Finance, brapi e Mercado Bitcoin |
| **Sai do computador?** | Só o código do ativo, para buscar a cotação. Quantidades e valores não saem |
| **Por quanto tempo** | Até você apagar |
| **Como apagar** | Em **Investimentos**: excluir a operação ou o ativo |

### Erros do sistema

| | |
|---|---|
| **O que é** | `ErroSistema`: mensagem, detalhe técnico, rota e usuário de cada erro ou aviso |
| **De onde vem** | Do próprio sistema |
| **Sai do computador?** | Não |
| **Por quanto tempo** | 90 dias pela última ocorrência, por padrão (de 7 a 365, em Parâmetros) |
| **Como apagar** | Sozinho, pelo prazo |

Antes de gravar, o `Sanitizador` tira o que está no padrão de segredo, CPF e valor em reais ([ADR 0022](adr/0022-erros-do-sistema-gravados-no-banco.md)). É uma limpeza por padrão de texto, e não uma garantia.

### Logs do serviço

| | |
|---|---|
| **O que é** | O registro do processo: no Windows, avisos e erros no Visualizador de Eventos; no Linux, o `journald` |
| **De onde vem** | Do próprio sistema |
| **Onde fica** | Fora da pasta `dados`, no sistema operacional |
| **Sai do computador?** | Não |
| **Por quanto tempo** | O que o sistema operacional guardar |
| **Como apagar** | Pelas ferramentas do sistema operacional |

**O log não passa pelo `Sanitizador`.** Mensagens de erro podem chegar como vieram, e no Linux o nível padrão é mais detalhado que no Windows.

## O navegador

As preferências de exibição, como o tema e "ocultar valores", ficam no armazenamento do navegador ([ADR 0017](adr/0017-preferencias-de-exibicao-no-navegador.md)). O cookie de sessão expira sem uso.

## Cópias

Toda cópia do banco tem **tudo** o que estava nele quando foi feita:

| Cópia | Onde fica |
|---|---|
| Automática, antes de mudar a estrutura | `dados/backups` (as 5 mais recentes) |
| Pelo gerenciador | Onde você escolheu |
| Pelos scripts | `backups`, na pasta do projeto |

**Apagar um dado no site não o apaga das cópias.** Para que ele suma de vez, apague as cópias também. A cópia não é criptografada: guarde-a num lugar seguro.

## Apagar tudo

- **Windows, pelo `.msi`:** `msiexec /x FinancasLocal-<versão>.msi APAGARDADOS=1`. Desinstalar sem essa opção mantém os dados.
- **Linux, pelos scripts:** pare o serviço e apague as pastas `dados` e `backups` do projeto.
- **Nos dois:** apague as cópias que você salvou fora daí, e a conexão no [Meu Pluggy](https://meu.pluggy.ai), se não for mais usar.
