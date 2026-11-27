### Correções
- **A tabela nova do INSS e do IRRF chega a quem atualiza pelo instalador** (#805). Antes, o cálculo do holerite, do 13º, das férias e da rescisão seguia com a tabela do ano anterior; agora a tabela nova entra sozinha ao abrir o site, sem apagar nem recriar nada seu.
  - Técnico: as tabelas oficiais saem do `02-dados-iniciais.sql` para o `03-tabelas-oficiais.sql`, aplicado a cada partida; ADR 0065.
