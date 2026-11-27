### Correções
- **Trabalho e Módulos: pontos da revisão** (#750). Sem ler os módulos, o menu avisa, com "Tentar de novo". "Reabrir" não apaga mais o holerite fechado de um mês sem salário ou fora do emprego. No 13º, trocar só a data da 1ª parcela grava, e as horas dos meses sem avo (admissão e saída) saem da média.
  - Técnico: `ModulosService.carregando`; ligar o módulo já ligado não roda a tarefa, e o intervalo recomeça do ciclo do sinal; `dataInformada` na parcela; `MesesComAvo`; ADRs 0064 e 0065.
