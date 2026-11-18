### Correções
- **Trabalho e Módulos: pontos da revisão** (#750). Se o site não consegue ler os módulos, o menu avisa no topo, com "Tentar de novo". "Reabrir" não apaga mais o holerite fechado de um mês que ficou sem salário ou fora do emprego. No 13º, trocar só a data da 1ª parcela grava, e as horas do mês da admissão sem avo saem da média.
  - Técnico: `ModulosService.carregando`; ligar o módulo já ligado não roda a tarefa, e o intervalo recomeça do ciclo do sinal; `dataInformada` na parcela; `MesesComAvo`; ADRs 0064 e 0065.
