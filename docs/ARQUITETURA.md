# Arquitetura

```
Navegador (Angular)  ──HTTPS 127.0.0.1:5443──►  Financas.Api (.NET 10, Kestrel)
                                                   │
                                                   ├─► SQLite (dados/sql/FinancasLocal.db, arquivo local)
                                                   ├─► api.pluggy.ai          (Open Finance)
                                                   ├─► api.bcb.gov.br         (Selic)
                                                   ├─► olinda.bcb.gov.br      (câmbio — PTAX)
                                                   └─► Yahoo Finance/brapi    (cotações)
```

A API sobe direto pelo Kestrel (sem IIS nem proxy) e roda como serviço do sistema: `UseSystemd` no Linux e `UseWindowsService` no Windows, com os mesmos scripts bash nos dois.

O Angular é publicado dentro da própria API (`wwwroot`), então site e API têm a mesma origem: sem CORS, cookies `__Host-` e proteção CSRF simples.

## Back-end — `src/backend/Financas.Api`

| Pasta | Responsabilidade |
|---|---|
| `Controllers/` | Endpoints REST (`/api/...`). Finos: validam a entrada e chamam os serviços. `BaseController` padroniza erros. Um controller pode reunir recursos relacionados (ex.: `PlanejamentoControllers.cs` com orçamentos, metas e contas a pagar). |
| `Servicos/` | Regras de negócio. `RelatorioService` (painel, orçamentos, projeção, visão anual), `CategorizacaoService` (regras automáticas), `SugestaoCategoriaService` (sugestões pelo histórico), `SugestaoContaAgendadaService` (contas a pagar recorrentes achadas no extrato), `ContaService` (saldos, faturas do mês dos cartões), `AssistenteMetasService` (caixa mês a mês para o plano das metas: receitas − despesas − faturas que vencem; a divisão entre as metas fica no front-end, em `metas-plano.ts`, [ADR 0035](adr/0035-plano-das-metas-pelo-caixa-calculado-na-tela.md)), `BancoService` (banco de cada conta: catálogo e detecção pelo nome, [ADR 0024](adr/0024-banco-de-cada-conta-com-selo-local.md)), `TipoLancamentoService` (tipo de cada lançamento: Pix, compra no cartão, pagamento de fatura…, [ADR 0025](adr/0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md)), `EmprestimoService` (empréstimos e financiamentos), `ConfiguracaoService`, `AuditoriaService`, `Relogio` (fuso de Brasília). |
| `Servicos/Pluggy/` | Cliente HTTP da Pluggy (sem SDK), modelos e sincronização (agendada pelo `SincronizacaoHostedService`). |
| `Servicos/Investimentos/` | Carteira de renda variável, cotações (`CotacaoService`), Selic (`SelicService`), FIIs (`FiiService`), importação pelo extrato (`ExtratoCarteiraService`), conferência com a corretora e o extrato (`ConferenciaInvestimentosService`, ADR 0027) e reservas estimadas pelo extrato (`ReservaService`, que também estima o Méli Dólar em dólar, ADR 0038). |
| `Servicos/Cambio/` | Cotação PTAX de moedas estrangeiras (`CambioService`), guardada no banco para uso offline e atualizada pelo `CambioHostedService`. |
| `Servicos/Diagnostico/` | Erros do sistema ([ADR 0022](adr/0022-erros-do-sistema-gravados-no-banco.md)): `RegistroErrosLoggerProvider` (avisos e erros do `ILogger` das classes `Financas.*`), `FilaErros`, `GravadorErrosHostedService` (grava na tabela `ErroSistema`, agrupa repetições e apaga os antigos) e `Sanitizador` (tira tokens, senhas, CPF e valores antes de gravar). |
| `Entidades/` | Classes mapeadas no EF Core. |
| `Data/FinancasDbContext.cs` | Mapeamento das tabelas (nomes, tamanhos, precisão, índices) — em sincronia manual com `banco/01-estrutura.sql`. |
| `Dtos/` | Contratos de entrada e saída da API (arquivo único `Dtos.cs`). |
| `Seguranca/` | Cabeçalhos de segurança (CSP), validação de sessão, constantes de cookies. |

Decisões (o porquê de cada uma, com alternativas e custos, está nas [ADRs](adr/README.md)):
- **SQLite + Kestrel**, sem SQL Server nem IIS ([ADR 0002](adr/0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md)).
- **Sem migrations.** O banco é criado e atualizado por scripts SQL idempotentes ([ADR 0003](adr/0003-esquema-por-scripts-sql-idempotentes.md); passo a passo em [BANCO-DE-DADOS.md](BANCO-DE-DADOS.md)).
- **Front-end no `wwwroot` da API**, mesma origem ([ADR 0004](adr/0004-front-end-publicado-no-wwwroot-da-api.md)).
- **Cliente HTTP próprio** para a Pluggy e as demais integrações ([ADR 0006](adr/0006-cliente-http-proprio-para-a-pluggy.md)).
- **Câmbio pela PTAX** do Banco Central, guardado no banco para uso offline ([ADR 0007](adr/0007-cotacao-de-cambio-pela-ptax-do-banco-central.md)). Só investimentos respeitam moeda estrangeira; o resto é em reais ([ADR 0008](adr/0008-moeda-estrangeira-somente-em-investimentos.md)).
- **Reservas sem saldo no Open Finance** (cofrinhos do Mercado Pago) são estimadas pelos lançamentos "Dinheiro reservado/retirado" do extrato, com conferência manual opcional do saldo real ([ADR 0010](adr/0010-reservas-estimadas-pelo-extrato.md)).
- **Consultas só com LINQ/lambda** sobre entidades mapeadas; atualizações em massa com `ExecuteUpdate`/`ExecuteDelete`.
- **Segredos** (Client Secret da Pluggy, token da brapi) ficam criptografados na tabela `Configuracao` com Data Protection. Chaves em `dados/chaves`: protegidas por DPAPI no Windows e por permissão de arquivo no Linux ([ADR 0005](adr/0005-segredos-criptografados-com-data-protection.md)).
- **Datas** sempre no fuso de Brasília (`Relogio`).
- **Acesso ao banco** é só permissão de arquivo do sistema sobre `dados/`: o SQLite não tem login nem usuário; quem lê e grava é o processo da API, rodando com a conta do serviço. Não há senha de banco em lugar nenhum.

## Front-end — `src/frontend`

| Pasta | Responsabilidade |
|---|---|
| `src/app/core/` | `ApiService` (todas as chamadas HTTP), modelos (interfaces TypeScript), autenticação, tema, privacidade. |
| `src/app/compartilhado/` | Componentes reutilizáveis: gráfico (Chart.js), modal, paginação, calculadora, pipe de moeda que respeita "ocultar valores", campo de valor em dinheiro (`app-campo-valor`, [ADR 0032](adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)). |
| `src/app/layout/` | Estrutura da tela (menu lateral). |
| `src/app/paginas/` | Uma página por rota. |
| `src/styles.css` | Tema (tokens de cor claros/escuros) e componentes visuais globais. |
| `testes/` | Testes com Jest ([ADR 0013](adr/0013-testes-do-front-end-com-jest.md)): `regressao/`, `usabilidade/` e `apoio/` (API falsa, abertura de telas pelo roteador). Fora do build do Angular. |

Angular 22 com zone.js (`provideZoneChangeDetection`) e componentes standalone. Todo componente declara `changeDetection: ChangeDetectionStrategy.Eager` — o comportamento padrão até o Angular 21 —, porque várias telas guardam estado em campos comuns e não em signals; o padrão OnPush do Angular 22 deixaria essas telas sem atualizar.

## Ferramentas — `src/backend/Financas.MigrarSqlite`

Migra os dados de uma instalação antiga (SQL Server/IIS) para o SQLite. É seguro rodar sem SQL Server disponível: sem nada para migrar, termina sem erro. O Client Secret da Pluggy e o token da brapi são limpos na migração (foram criptografados com as chaves da instalação antiga) e precisam ser informados de novo.

```bash
dotnet run --project src/backend/Financas.MigrarSqlite -- "<connection string SQL Server origem>" "<caminho .db destino>"
```

## Instalador do Windows — `instalador/`

`.msi` gerado com o WiX 5 pela CI (`.github/workflows/versao.yml`; [ADR 0029](adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)):
- **Conteúdo:** API e gerenciador já compilados (self-contained `win-x64`) em `%ProgramFiles%\FinancasLocal`.
- **Dados:** em `%ProgramData%\FinancasLocal\dados`, que é o `Dados:Pasta` do `appsettings.Production.json` do pacote.
- **Serviço:** `FinancasLocal` (LocalSystem).
- **Ação personalizada (`configurar.ps1`):** pasta de dados protegida, migração da instalação pelos scripts, certificado HTTPS e `configuracao.json`.
- **Gerar o pacote:** `gerar-msi.ps1` (`-Revisao N` para a versão de desenvolvimento `X.Y.Z-dev.N`).
- **Onde baixar:** a versão publicada fica no Release `vX.Y.Z`. A do `develop` fica no pré-lançamento `desenvolvimento`, recriado a cada push depois dos testes. A mesma `X.Y.Z` substitui a instalada (`AllowSameVersionUpgrades`).
- **Gerenciador:** `src/gerenciador/Financas.Gerenciador` ([ADR 0028](adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md)).

## Gerenciador — `src/gerenciador/`

App pequeno que mostra se o site está no ar, liga, desliga e reinicia o serviço e copia o banco. Roda como o usuário, fora da API ([ADR 0028](adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md), [ADR 0033](adr/0033-gerenciador-no-linux-com-avalonia.md)).

| Projeto | O que tem |
|---|---|
| `Financas.Gerenciador.Nucleo` (`net10.0`) | O que vale nos dois sistemas: `Instalacao` (pasta da API pelo serviço, banco e endereço pelo `appsettings`), `VerificadorSituacao` (serviço + `GET /api/auth/estado`), `CopiaDoBanco` (`VACUUM INTO`) e o serviço do sistema (`ServicoWindows`, `ServicoLinux`). |
| `Financas.Gerenciador` (WinForms) | Janela do Windows. Pede administrador (UAC) para ligar, desligar e copiar o banco, e atualiza pelo `.msi` do GitHub Releases. Vai no `.msi` ou pelo `scripts\windows\instalar-gerenciador.ps1`. |
| `Financas.Gerenciador.Linux` (Avalonia) | Janela do Linux. Usa o `systemctl --user` sem senha, mostra o registro do serviço (journald) e o comando para atualizar pelos scripts. Vai pelo `scripts/linux/instalar-gerenciador.sh`. |

## Scripts — `scripts/`

Separados pelo sistema em que rodam (#154): `geral/` é o bash que roda nos dois (no Windows, pelo Git Bash/MSYS ou WSL); `windows/` e `linux/` têm o que só funciona num deles. No Windows, a instalação também tem uma versão em PowerShell que dispensa o bash ([ADR 0012](adr/0012-instalador-do-windows-em-powershell.md)). Os `.cmd` (`instalar-windows.cmd`, `parar-windows.cmd`) ficam na raiz, porque são a porta de entrada de quem não usa terminal.

| Script | Função |
|---|---|
| `geral/instalar.sh` | Roda as etapas 1, 2 e 3 em sequência e para no primeiro erro. |
| `geral/1-preparar-ambiente.sh` | Confere as ferramentas básicas do terminal; instala Node.js LTS, .NET 10 SDK e, se o sistema não tiver, `sqlite3` portáteis em `ferramentas/` (reaproveita um `dotnet` 10.x e um `sqlite3` do PATH). Não depende de Python ([ADR 0011](adr/0011-ferramentas-de-instalacao-portateis-sem-python.md)). |
| `geral/2-criar-banco.sh` | Cria as pastas de `dados/` com permissão restrita e aplica os arquivos de `banco/` em ordem. Também é lido pela API (embutido no build, `EsquemaBanco`) e pelo `instalar-windows.ps1`, que tiram dele a lista de colunas novas. |
| `geral/3-publicar-atualizar.sh` | Compila front e API, publica em `publicado/api`, gera/reaproveita o certificado HTTPS local e registra o serviço (systemd `--user` no Linux, `sc.exe` no Windows). |
| `geral/4-backup.sh` | Backup online do banco (`sqlite3 .backup`), com retenção e opção de agendar. |
| `geral/parar.sh` | Para o serviço, a API fora dele e o `npm start`; no Git Bash, repassa para o `windows/parar-windows.ps1`. |
| `geral/comum.sh` | Funções compartilhadas pelos scripts (detecção de SO, pastas, `sqlite3` portátil no PATH, caminho nativo no Windows, extração de `.zip`, conferência de hash, helpers de coluna do banco). |
| `windows/instalar-windows.ps1` | Etapas 1, 2 e 3 no Windows PowerShell 5.1, sem bash; chamado por `instalar-windows.cmd` (na raiz), que pede elevação. Lê a lista de colunas novas do `geral/2-criar-banco.sh`. Mudanças nas etapas 1 e 3 do bash precisam ser refletidas aqui. |
| `windows/parar-windows.ps1` | O `parar.sh` em PowerShell; chamado por `parar-windows.cmd` (na raiz). |
| `windows/instalar-gerenciador.ps1`, `linux/instalar-gerenciador.sh` | Opcionais, sem administrador/root: publicam o gerenciador em `publicado/gerenciador` e criam os atalhos (Menu Iniciar no Windows, menu de aplicativos no Linux, e a área de trabalho). Rodar de novo atualiza o gerenciador. |
| `windows/configurar-github.ps1` | Cria o repositório no GitHub, envia as branches e aplica as proteções. |
| `ci/verificar-sql.py`, `ci/verificar-manual.py`, `ci/verificar-versao.py` | Usados na CI: banco idempotente e sem T-SQL, aviso de Manual desatualizado e versão subiu (ADR 0039). |
| `3-publicar-atualizar.sh`, `4-backup.sh`, `instalar-gerenciador.sh` (na raiz de `scripts/`) | **Repasses temporários** para os caminhos novos: o backup agendado antes da mudança e o comando que o gerenciador do Linux antigo mostra continuam funcionando. Podem ser apagados numa versão futura. |
