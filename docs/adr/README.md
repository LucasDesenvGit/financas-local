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

As ADRs 0002 a 0006 foram escritas depois das decisões, a partir do código e da documentação existentes. O contexto delas é reconstruído; o que não estava registrado está marcado como *inferido*.

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
| [0010](0010-reservas-estimadas-pelo-extrato.md) | Reservas estimadas pelo extrato, com conferência manual | Aceita |
| [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) | Ferramentas de instalação portáteis, sem depender de Python | Aceita |
| [0012](0012-instalador-do-windows-em-powershell.md) | Instalador do Windows em PowerShell, sem bash | Aceita |
