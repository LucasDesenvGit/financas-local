# Dados pessoais

O que o Finanças Local guarda, onde, o que sai do computador, por quanto tempo fica e como apagar.

> Este é um documento técnico de transparência, conferido no código. Não é uma política de privacidade jurídica. A LGPD serve de roteiro: o sistema roda no seu computador, e você é quem controla os dados.

## Em resumo

- **Tudo fica no seu computador**, na pasta `dados`. Não há servidor do projeto: ninguém além de você recebe esses dados.
- **O que sai** vai para os serviços da [tabela de saídas](SEGURANCA.md#o-que-sai-para-a-internet), sempre o mínimo. Quantidades, valores e dados pessoais não saem.
- **Para quem o sistema traz dados:** a Pluggy entrega extratos, saldos e investimentos dos bancos que você autorizou no Meu Pluggy.
- **O banco não é criptografado.** Só os segredos são. Por isso, a recomendação é criptografar o disco (BitLocker ou LUKS). De quem o sistema se protege: [modelo de ameaças](MODELO-DE-AMEACAS.md).

## Por grupo de dados

Todos os grupos ficam no banco (`dados/sql/FinancasLocal.db`) e também em **todas as [cópias](#cópias)**.

### Login e auditoria

| | |
|---|---|
| **O que é** | `Usuario`: login, hash da senha (PBKDF2), tentativas e último login. `AuditoriaAcesso`: evento, login, IP e navegador de cada ação registrada (login, senha, parâmetros, credenciais, conexões, alterações em lote, conferências e o acesso pelo celular ligado e desligado). O IP é `127.0.0.1` no computador e, com o acesso pelo celular ligado, o do aparelho na rede de casa |
| **De onde vem** | De você, e do próprio sistema |
| **Sai do computador?** | Não |
| **Por quanto tempo** | `Usuario`: enquanto houver o login. `AuditoriaAcesso`: 180 dias, ajustável de 30 a 730 em Configurações › Parametrização › Segurança (“Guardar o histórico de acessos por”, #530) |
| **Como apagar** | A auditoria mais antiga que o prazo é apagada sozinha, ao subir o site e uma vez por dia. Para apagar antes, diminua o prazo. O `Usuario`, só apagando tudo |

### Credenciais

| | |
|---|---|
| **O que é** | `Configuracao`: o Client ID da Pluggy, em texto, e o Client Secret da Pluggy e o token da brapi, criptografados com as chaves de `dados/chaves` |
| **De onde vem** | De você: as da Pluggy em **Configurações › Open Finance**; o token da brapi em **Investimentos › Renda variável › fonte das cotações** |
| **Sai do computador?** | Sim: as credenciais da Pluggy vão para `api.pluggy.ai`, e o token, para `brapi.dev` (no cabeçalho `Authorization`), por HTTPS |
| **Por quanto tempo** | Até você trocar ou remover |
| **Como apagar** | Remova no mesmo lugar em que informou. A API nunca devolve o segredo |

### Seu nome nos Pix

| | |
|---|---|
| **O que é** | `Configuracao` (`Transferencias.MeuNome`): "Seu nome como aparece nos Pix", em texto. Serve para reconhecer, nos bancos que não informam o titular, o Pix ou a TED para outra conta sua ([ADR 0031](adr/0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md)) |
| **De onde vem** | De você, em **Configurações › Open Finance**, se quiser |
| **Sai do computador?** | Não |
| **Por quanto tempo** | Até você trocar ou apagar |
| **Como apagar** | Esvazie o campo e clique em **Salvar nome** |

### Acesso pelo celular

| | |
|---|---|
| **O que é** | `Configuracao` (`AcessoCelular.Ip`): o IP deste computador na rede de casa, escolhido ao ligar o acesso pelo celular ([ADR 0061](adr/0061-acesso-pelo-celular-na-rede-local.md)). Fora do banco, em `dados/chaves`, a autoridade que assina o certificado do celular (`celular-autoridade.chave`, a chave, e `celular-autoridade.crt`, a parte pública): não é dado pessoal, mas é chave a proteger como as outras |
| **De onde vem** | De você, em **Configurações › Segurança**, se ligar. A autoridade, do próprio sistema, na primeira vez que liga |
| **Sai do computador?** | Não para a internet. Com o acesso ligado, o site responde aos aparelhos da sua rede de casa, que veem o que você abrir neles depois de entrar. A parte pública da autoridade vai para o celular que a baixar |
| **Por quanto tempo** | O IP, até desligar. A autoridade, até você apagar os dois arquivos (ela vale 10 anos) |
| **Como apagar** | **Desligar o acesso pelo celular** apaga o IP e a regra do firewall do Windows. A autoridade fica, para o celular não precisar instalar de novo; para trocá-la, com o acesso desligado, apague os dois arquivos `celular-autoridade` (e remova a antiga do celular) |

### Dados bancários da Pluggy

| | |
|---|---|
| **O que é** | `ConexaoBancaria`: a instituição e o identificador na Pluggy. `Conta`: nome, número como o banco informa, saldo e limite. `FaturaCartao`: valores e datas. `Transacao`: descrição, descrição original, estabelecimento, valor e data. `Investimento`, `HistoricoInvestimento`, `Emprestimo` e `PagamentoEmprestimo`: nome, emissor, saldos e parcelas. `SincronizacaoLog`: quando rodou e a mensagem |
| **De onde vem** | Do Open Finance, pela Pluggy, dos bancos que você autorizou |
| **Sai do computador?** | Não. Esses dados só entram |
| **Por quanto tempo** | Sem prazo, enquanto a conexão existir |
| **Como apagar** | **Remover a conexão** em Configurações apaga tudo o que veio dela: as contas, transações, faturas, investimentos e empréstimos, o histórico dos investimentos e o registro das sincronizações. O registro de uma sincronização de todas as conexões juntas, que não é de nenhuma delas, fica. Uma transação do Open Finance sozinha não pode ser excluída; dá para marcá-la para ignorar nos relatórios |

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
| **De onde vem** | De você, do extrato e do Open Finance. As cotações e os proventos vêm do Yahoo Finance e da brapi |
| **Sai do computador?** | Só o código do ativo, para buscar a cotação. Quantidades e valores não saem |
| **Por quanto tempo** | Até você apagar |
| **Como apagar** | Em **Investimentos**: excluir a operação ou o ativo |

### Trabalho: emprego e salário (módulo)

| | |
|---|---|
| **O que é** | `Emprego`: empregador, CNPJ, admissão e saída, regime, jornada, dependentes para o IR e a conta onde o salário cai. `SalarioVigencia`: o salário bruto de cada período. `Holerite` e `HoleriteLancamento`: o holerite de cada mês (horas extras, bônus, descontos como plano de saúde e pensão alimentícia) e, fechado, o cálculo guardado. `DecimoTerceiro`: quando e quanto veio a 1ª parcela do 13º. `FeriasFaltas` e `FeriasAgendamento`: as faltas sem justificativa de cada período e as férias agendadas (datas, dias, dias vendidos) ([ADR 0065](adr/0065-trabalho-emprego-salario-com-vigencia-e-tabelas-oficiais.md)). As tabelas do INSS e do IRRF (`InssVigencia`, `InssFaixa`, `IrrfVigencia`, `IrrfFaixa`) são públicas, não pessoais |
| **De onde vem** | De você, em **Trabalho › Salário** e **Trabalho › Holerites**, com o módulo Trabalho ligado (Configurações › Módulos) |
| **Sai do computador?** | Não. O módulo não acessa a internet |
| **Por quanto tempo** | Até você apagar. Desligar o módulo esconde a tela, mas guarda os dados |
| **Como apagar** | Em **Trabalho › Salário**, **Editar emprego** › **Excluir emprego** apaga o emprego, o histórico de salários e os holerites dele; **Apagar** tira um salário do histórico. Em **Trabalho › Holerites**, **Excluir** apaga o holerite aberto (o fechado, reabra antes) |

### Cotações, índices e categorias públicas

| | |
|---|---|
| **O que é** | `CotacaoMoeda` (PTAX), `CotacaoCripto`, `TaxaSelicDiaria`, `IndiceMensal` (como o IFIX) e `CategoriaPluggy` (as categorias que a Pluggy usa e a sua categoria para cada uma). Não são dados seus, a não ser a escolha da categoria |
| **De onde vem** | Do Banco Central, do Mercado Bitcoin, do Yahoo Finance e da brapi, e da Pluggy |
| **Sai do computador?** | Não: só entram. O que sai para buscá-los está na [tabela de saídas](SEGURANCA.md#o-que-sai-para-a-internet) |
| **Por quanto tempo** | Sem prazo |
| **Como apagar** | Só apagando tudo |

### Erros do sistema

| | |
|---|---|
| **O que é** | `ErroSistema`: mensagem, detalhe técnico, rota e usuário de cada erro ou aviso |
| **De onde vem** | Do próprio sistema |
| **Sai do computador?** | Não |
| **Por quanto tempo** | 90 dias pela última ocorrência, por padrão (de 7 a 365, em **Configurações › Parametrização**) |
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

**O log passa pela mesma limpeza dos erros gravados no banco** (#531): o que está no padrão de segredo, CPF ou valor em reais sai como `[removido]`, na mensagem e nos atributos. É uma limpeza por padrão de texto, e não uma garantia. No Linux, o `journald` recebe também as informações; no Windows, o Visualizador de Eventos recebe só os avisos e os erros.

## O navegador

As preferências de exibição, como o tema e "ocultar valores", ficam no armazenamento do navegador ([ADR 0017](adr/0017-preferencias-de-exibicao-no-navegador.md)). O cookie de sessão expira sem uso.

## Cópias

Toda cópia do banco tem **tudo** o que estava nele quando foi feita:

| Cópia | Onde fica |
|---|---|
| Automática, antes de mudar a estrutura | `dados/backups` (as 5 mais recentes) |
| Pelo gerenciador | Onde você escolheu |
| O banco de antes de restaurar uma cópia (#533) | `dados/antes-de-restaurar`, até você apagar |
| Pelos scripts | `backups`, na pasta do projeto |

**Apagar um dado no site não o apaga das cópias.** Para que ele suma de vez, apague as cópias também. A cópia não é criptografada: guarde-a num lugar seguro.

## Apagar tudo

- **Windows, pelo `.msi`:** `msiexec /x FinancasLocal-<versão>.msi APAGARDADOS=1`. Desinstalar sem essa opção mantém os dados.
- **Linux, pelos scripts:** pare o serviço e apague as pastas `dados` e `backups` do projeto.
- **Nos dois:** apague as cópias que você salvou fora daí, e a conexão no [Meu Pluggy](https://meu.pluggy.ai), se não for mais usar.
