# Registros de decisão de arquitetura (ADRs)

Cada ADR registra **uma** decisão técnica que afeta a forma do sistema: o problema, as alternativas consideradas, o que foi escolhido e o custo dessa escolha. O objetivo é que quem chegar depois (inclusive o Claude Code) entenda **por que** o código é como é antes de mudar.

## Quando escrever uma ADR

Escreva (ou atualize) uma ADR quando a mudança:

- adiciona ou troca uma **dependência, biblioteca ou integração externa** (nova API, novo domínio de saída de rede);
- muda o **banco** além de uma tabela/coluna rotineira (motor, estratégia de esquema, cache, forma de guardar um tipo de dado);
- muda **segurança**: autenticação, segredos, CSP, exposição de rede, permissões;
- muda **infraestrutura** ou publicação: hospedagem, serviço do sistema, scripts, CI;
- estabelece ou quebra um **padrão de código** que vale para o projeto todo (camadas, nomes, forma de chamar a API).

Não precisa de ADR: correção de bug, refatoração local, ajuste visual, nova tela ou endpoint que só segue padrões já registrados.

## Como escrever

1. Copie [0000-modelo.md](0000-modelo.md) para `NNNN-titulo-em-kebab-case.md`, com o próximo número livre.
2. Preencha contexto, alternativas, decisão e consequências — **as consequências negativas são obrigatórias**.
3. Adicione a linha no índice abaixo.
4. Inclua a ADR no mesmo pull request da mudança que ela justifica.

## Ciclo de vida

| Status | Significado |
|---|---|
| Proposta | Em discussão (pull request aberto). |
| Aceita | Vale para o código atual. |
| Substituída por NNNN | Trocada por outra decisão. A ADR antiga **não é apagada**: muda o status e aponta para a nova. |
| Descontinuada | Deixou de valer sem substituta (ex.: a funcionalidade foi removida). |

Uma ADR aceita só é **editada** para correções, novos detalhes da mesma decisão ou atualização de consequências. Se a decisão em si mudou, crie uma ADR nova e marque a antiga como substituída.

As ADRs 0002 a 0006 e 0014 a 0021 foram escritas depois das decisões, a partir do código, da documentação e dos pull requests que as introduziram (citados em cada uma). O contexto delas é reconstruído; o que não estava registrado está marcado como *inferido*.

## Índice

| ADR | Título | Status |
|---|---|---|
| [0001](0001-registrar-decisoes-em-adrs.md) | Registrar decisões de arquitetura em ADRs | Aceita |
| [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) | SQLite e Kestrel no lugar de SQL Server Express e IIS | Aceita (em parte substituída pela 0012) |
| [0003](0003-esquema-por-scripts-sql-idempotentes.md) | Esquema do banco por scripts SQL idempotentes, sem migrations | Aceita |
| [0004](0004-front-end-publicado-no-wwwroot-da-api.md) | Front-end publicado dentro da API (mesma origem) | Aceita |
| [0005](0005-segredos-criptografados-com-data-protection.md) | Segredos criptografados no banco com ASP.NET Data Protection | Aceita |
| [0006](0006-cliente-http-proprio-para-a-pluggy.md) | Cliente HTTP próprio para a Pluggy, sem SDK | Aceita |
| [0007](0007-cotacao-de-cambio-pela-ptax-do-banco-central.md) | Cotação de câmbio pela PTAX do Banco Central, guardada para uso offline | Aceita |
| [0008](0008-moeda-estrangeira-somente-em-investimentos.md) | Moeda estrangeira só em investimentos; demais registros em reais | Aceita (em parte substituída pela 0009) |
| [0009](0009-ativos-no-exterior-na-carteira-manual.md) | Ativos no exterior na carteira manual | Aceita |
| [0010](0010-reservas-estimadas-pelo-extrato.md) | Reservas estimadas pelo extrato, com conferência manual | Substituída por 0037 |
| [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) | Ferramentas de instalação portáteis, sem depender de Python | Aceita |
| [0012](0012-instalador-do-windows-em-powershell.md) | Instalador do Windows em PowerShell, sem bash | Aceita (em parte substituída pela 0029) |
| [0013](0013-testes-do-front-end-com-jest.md) | Testes automatizados do front-end com Jest | Aceita |
| [0014](0014-emprestimos-e-financiamentos-pelo-open-finance.md) | Empréstimos e financiamentos pelo Open Finance, separados e fora das despesas | Aceita |
| [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) | Parâmetros do sistema no banco, editáveis pelo site | Aceita |
| [0016](0016-icones-do-lucide-empacotados-no-build.md) | Ícones do Lucide empacotados no build | Aceita |
| [0017](0017-preferencias-de-exibicao-no-navegador.md) | Preferências de exibição ficam no navegador | Aceita |
| [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) | Estado da tela na URL e detalhe por navegação | Aceita |
| [0019](0019-status-da-sincronizacao-por-consulta-periodica.md) | Status da sincronização por consulta periódica a um endpoint leve | Aceita |
| [0020](0020-glossario-unico-com-balao-na-top-layer.md) | Glossário único, com o balão na top layer (Popover API) | Aceita |
| [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) | Componentes padrão para gráficos e telas densas | Aceita |
| [0022](0022-erros-do-sistema-gravados-no-banco.md) | Erros do sistema gravados no banco e vistos no site | Aceita |
| [0023](0023-edicao-em-massa-no-modo-edicao-de-transacoes.md) | Edição de categorias em massa como "modo edição" de Transações, com desfazer guardado na API | Aceita |
| [0024](0024-banco-de-cada-conta-com-selo-local.md) | Banco de cada conta e cartão, com selo desenhado no próprio site | Aceita |
| [0025](0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md) | Tipo de lançamento: dado do Open Finance, com a descrição como reserva | Aceita |
| [0026](0026-manual-do-site-como-dados-no-front-end.md) | Manual do site como dados no front-end, com testes que o mantêm em dia | Aceita |
| [0027](0027-conferencia-de-investimentos-com-previa-e-desfazer.md) | Conferência de investimentos: correções sugeridas pelo extrato, aplicadas só com confirmação | Aceita |
| [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) | Gerenciador do Windows, com atualização pelo GitHub Releases | Aceita |
| [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) | Instalador .msi para Windows, com WiX, gerado e testado pela CI | Aceita |
| [0030](0030-regras-de-descricao-em-tabela-propria.md) | Regras de descrição em tabela própria, com o nome do banco sempre guardado | Aceita |
| [0031](0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md) | Transferência só fica fora dos relatórios quando é entre contas suas | Aceita |
| [0032](0032-controles-padrao-interruptor-e-campo-de-valor.md) | Controles padrão: interruptor, caixa de marcar, campo de valor, campo de data e lista suspensa | Aceita |
| [0033](0033-gerenciador-no-linux-com-avalonia.md) | Gerenciador no Linux, com Avalonia e núcleo comum ao do Windows | Aceita |
| [0034](0034-sem-sri-no-build-do-front-end.md) | Build do front-end sem SRI, para manter a CSP sem script inline | Aceita |
| [0035](0035-plano-das-metas-pelo-caixa-calculado-na-tela.md) | Plano das metas pelo caixa do mês (fatura que vence entra), com a divisão calculada na tela | Aceita |
| [0036](0036-receita-e-despesa-pelo-tipo-estorno-abate-gasto.md) | Receita e despesa pelo tipo, não pelo sinal; estorno no cartão abate o gasto | Aceita |
| [0037](0037-reservas-pelo-valor-do-open-finance.md) | Reservas pelo valor que o Open Finance informa; a estimativa pelo extrato fica para o resto | Aceita |
