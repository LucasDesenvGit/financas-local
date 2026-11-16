import {
  ajustesParaUrl, alternarNoConjunto, aoMudarOAlvo, comAjuste, dataAlvoPeloPorMes, dtoDaMeta, faltaNoForm,
  fechadasAoAlternarTodas, formularioDaMeta, formularioVazio, graficoDaEvolucao, hojeDaEvolucao, lerAjustes, lerFechadas,
  limiteDoPorMes, mensagemLimitePorMes, mesDe, mesesAposMostrarMais, nomeMes, nomeMesCurto, opcoesCofrinho, ordemAposMover,
  porMesAcimaDaSobra, porMesAcimaDoLimite, posicaoNaOrdem, prazoVencido, previsaoPorMes, semAjuste
} from '../../../src/app/paginas/planejamento/metas';
import { EvolucaoDaMeta } from '../../../src/app/paginas/planejamento/metas-evolucao';
import { planejarMetas } from '../../../src/app/paginas/planejamento/metas-plano';
import { Reserva } from '../../../src/app/core/modelos';
import { meta } from '../apoio/metas';
import { METAS, plano } from '../apoio/plano-metas';

/**
 * #732 (ADR 0059): a lógica da tela Metas em funções puras (`metas.ts`), testada sem montar a tela. Os mesmos
 * comportamentos também são conferidos pela tela nos outros testes de Metas; aqui o que cada função decide, sozinha.
 */
describe('Regressão: lógica pura da tela Metas (#732)', () => {
  describe('a simulação na URL', () => {
    it('lerAjustes e ajustesParaUrl são inversas; sem ajuste, o parâmetro sai da URL', () => {
      const lidos = lerAjustes('1:50,3:120.5');
      expect(ajustesParaUrl(lidos)).toBe('1:50,3:120.5');
      expect(ajustesParaUrl(new Map())).toBeNull();
    });

    it('comAjuste guarda o valor; vazio, negativo ou inválido volta à recomendação; não mexe no mapa de entrada', () => {
      const base = new Map([[1, 50]]);
      expect([...comAjuste(base, 2, 80)]).toEqual([[1, 50], [2, 80]]);
      expect([...comAjuste(base, 1, 0)]).toEqual([[1, 0]]);
      for (const ruim of [null, undefined, -1, NaN, Infinity]) expect([...comAjuste(base, 1, ruim)]).toEqual([]);
      expect([...base]).toEqual([[1, 50]]);
    });

    it('semAjuste tira só a meta pedida', () => {
      expect([...semAjuste(new Map([[1, 50], [2, 60]]), 1)]).toEqual([[2, 60]]);
    });
  });

  describe('as sanfonas lembradas no navegador', () => {
    it('lerFechadas aceita só uma lista de ids inteiros; texto ruim ou ausente vira "nenhuma fechada"', () => {
      expect([...lerFechadas('[1,2,"x",3.5]')]).toEqual([1, 2]);
      for (const ruim of [null, 'não é json', '{"a":1}', '7']) expect(lerFechadas(ruim).size).toBe(0);
    });

    it('alternarNoConjunto põe e tira o id, sem mexer no conjunto de entrada', () => {
      const base = new Set([1]);
      expect([...alternarNoConjunto(base, 2)]).toEqual([1, 2]);
      expect([...alternarNoConjunto(base, 1)]).toEqual([]);
      expect([...base]).toEqual([1]);
    });

    it('"Fechar todas" com alguma aberta; "Abrir todas" com todas fechadas', () => {
      expect([...fechadasAoAlternarTodas([1, 2, 3], true)]).toEqual([1, 2, 3]);
      expect(fechadasAoAlternarTodas([1, 2, 3], false).size).toBe(0);
    });
  });

  describe('prioridade, prazo e nomes', () => {
    it('posicaoNaOrdem conta de 1; fora da lista é 0', () => {
      expect(posicaoNaOrdem(METAS, 1)).toBe(1);
      expect(posicaoNaOrdem(METAS, 3)).toBe(3);
      expect(posicaoNaOrdem(METAS, 99)).toBe(0);
    });

    it('ordemAposMover troca com a vizinha; nas pontas e fora da lista não se mexe', () => {
      expect(ordemAposMover([1, 2, 3], 2, -1)).toEqual([2, 1, 3]);
      expect(ordemAposMover([1, 2, 3], 2, 1)).toEqual([1, 3, 2]);
      expect(ordemAposMover([1, 2, 3], 1, -1)).toBeNull();
      expect(ordemAposMover([1, 2, 3], 3, 1)).toBeNull();
      expect(ordemAposMover([1, 2, 3], 9, 1)).toBeNull();
    });

    it('prazoVencido: só quando a data alvo é antes do mês do plano', () => {
      const mes = { ano: 2026, mes: 10 };
      expect(prazoVencido('2026-09-01', mes)).toBe(true);
      expect(prazoVencido('2026-10-01', mes)).toBe(false);
      expect(prazoVencido('2025-12-01', mes)).toBe(true);
      expect(prazoVencido(null, mes)).toBe(false);
      expect(prazoVencido('2020-01-01', null)).toBe(false);
    });

    it('porMesAcimaDaSobra devolve a sobra do mês só quando o valor por mês passa dela (com folga de meio centavo)', () => {
      expect(porMesAcimaDaSobra(800, 450)).toBe(450);
      expect(porMesAcimaDaSobra(450.004, 450)).toBeNull();
      expect(porMesAcimaDaSobra(null, 450)).toBeNull();
      expect(porMesAcimaDaSobra(800, undefined)).toBeNull();
    });

    it('nomes dos meses e o "Mostrar mais" do plano', () => {
      expect(nomeMes(1)).toBe('Janeiro');
      expect(nomeMesCurto(11)).toBe('nov');
      expect(mesesAposMostrarMais(5, 12, 5)).toBe(10);
      expect(mesesAposMostrarMais(10, 12, 5)).toBe(12);
      expect(mesDe(new Date(2026, 10, 16))).toEqual({ ano: 2026, mes: 11 });
    });
  });

  describe('o formulário da meta', () => {
    it('a meta nova começa vazia, sem conta, na primeira cor e em andamento', () => {
      const f = formularioVazio();
      expect(f.id).toBeNull();
      expect(f.porMes).toBeNull();
      expect(f.deFora).toBe(0);
      expect(f.dados).toMatchObject({ nome: '', valorAlvo: 0, valorAtual: 0, dataAlvo: null, contaId: null, concluida: false, icone: null });
    });

    it('editar leva o valor inicial (não o saldo), o por mês gravado e o que vem de fora (conta ou cofrinho)', () => {
      const f = formularioDaMeta(meta({ id: 7, nome: 'Casa', valorAtual: 3750, valorInicial: 500, guardarPorMes: 100, icone: undefined }));
      expect(f).toMatchObject({ id: 7, porMes: 100, deFora: 3250 });
      expect(f.dados).toMatchObject({ nome: 'Casa', valorAtual: 500, icone: null });
    });

    it('dtoDaMeta manda números, data vazia como null e o por mês só acima de zero', () => {
      const f = formularioVazio();
      f.dados.valorAlvo = '1000' as unknown as number;
      f.dados.valorAtual = null as unknown as number;
      f.dados.dataAlvo = '';
      expect(dtoDaMeta(f)).toMatchObject({ valorAlvo: 1000, valorAtual: 0, dataAlvo: null, guardarPorMes: null });
      f.porMes = 250;
      f.dados.dataAlvo = '2027-01-01';
      expect(dtoDaMeta(f)).toMatchObject({ dataAlvo: '2027-01-01', guardarPorMes: 250 });
      f.porMes = 0;
      expect(dtoDaMeta(f).guardarPorMes).toBeNull();
    });

    const reserva = (nome: string, contaId: number, saldo: number) => ({ contaId, nome, saldo }) as unknown as Reserva;

    it('opcoesCofrinho: só os da conta, em ordem; o escolhido que sumiu do extrato continua na lista, sem saldo', () => {
      const todas = [reserva('Viagem', 1, 3250), reserva('Emergência', 1, 12400), reserva('Outro', 2, 1)];
      expect(opcoesCofrinho(todas, null, null)).toEqual([]);
      expect(opcoesCofrinho(todas, 1, null)).toEqual([{ nome: 'Emergência', saldo: 12400 }, { nome: 'Viagem', saldo: 3250 }]);
      expect(opcoesCofrinho(todas, 1, 'viagem').map((o) => o.nome)).toEqual(['Emergência', 'Viagem']);
      expect(opcoesCofrinho(todas, 1, 'Antigo')).toContainEqual({ nome: 'Antigo', saldo: null });
    });

    it('faltaNoForm: o alvo menos o já guardado e o que vem de fora, nunca abaixo de zero', () => {
      expect(faltaNoForm(6000, 1000, 0)).toBe(5000);
      expect(faltaNoForm(6000, 1000, 3250)).toBe(1750);
      expect(faltaNoForm(1000, 1000, 500)).toBe(0);
      expect(faltaNoForm(1000, null as unknown as number, 0)).toBe(1000);
    });

    it('aoMudarOAlvo: subir o alvo de uma concluída reabre a meta; descer, ou subir sem passar do guardado, não', () => {
      const concluida = formularioDaMeta(meta({ concluida: true, valorAlvo: 200, valorAtual: 200, valorInicial: 200 }));
      expect(aoMudarOAlvo(concluida, 30000)).toEqual({ concluida: false, reaberta: true });
      expect(aoMudarOAlvo(concluida, 100)).toEqual({ concluida: true, reaberta: false });
      expect(aoMudarOAlvo(concluida, 200)).toEqual({ concluida: true, reaberta: false });
      const emAndamento = formularioDaMeta(meta({ concluida: false, valorAlvo: 200, valorAtual: 200, valorInicial: 200 }));
      expect(aoMudarOAlvo(emAndamento, 30000)).toEqual({ concluida: false, reaberta: false });
    });
  });

  describe('o valor por mês', () => {
    const caixa = plano();
    const ativas = [meta({ id: 2, guardarPorMes: 1000 }), meta({ id: 3 })];
    const calculado = planejarMetas(caixa, ativas, 90);

    it('limiteDoPorMes: a maior parte da sobra, menos o que as outras metas já guardam (a editada não conta)', () => {
      // Sobra de R$ 3.500, 90% para as metas: R$ 3.150, menos R$ 1.000 da outra meta.
      expect(limiteDoPorMes(caixa, calculado, 90, ativas, null)).toBe(2150);
      expect(limiteDoPorMes(caixa, calculado, 90, ativas, 2)).toBe(3150);
      expect(limiteDoPorMes(caixa, calculado, 90, [meta({ id: 9, guardarPorMes: 99999 })], null)).toBe(0);
    });

    it('limiteDoPorMes: sem histórico (ou sem plano calculado) não há limite', () => {
      expect(limiteDoPorMes(plano({ disponivel: false }), null, 90, ativas, null)).toBeNull();
      expect(limiteDoPorMes(null, null, 90, ativas, null)).toBeNull();
      expect(limiteDoPorMes(caixa, null, 90, ativas, null)).toBeNull();
    });

    it('mensagemLimitePorMes diz o máximo, ou que não sobra nada', () => {
      expect(mensagemLimitePorMes(2150)).toContain('informe até R$ 2.150,00 por mês');
      expect(mensagemLimitePorMes(0)).toContain('Não sobra nada para as metas');
      expect(mensagemLimitePorMes(null)).toBe('');
    });

    it('porMesAcimaDoLimite: só com limite e valor maior que ele', () => {
      expect(porMesAcimaDoLimite(2150, 22200)).toBe(true);
      expect(porMesAcimaDoLimite(2150, 2150)).toBe(false);
      expect(porMesAcimaDoLimite(null, 22200)).toBe(false);
      expect(porMesAcimaDoLimite(2150, null)).toBe(false);
    });

    it('previsaoPorMes e dataAlvoPeloPorMes: a data alvo acompanha o valor por mês, dentro do limite', () => {
      const f = formularioVazio();
      f.dados.valorAlvo = 6000;
      f.dados.valorAtual = 1000;
      f.porMes = 1000;
      const hoje = { ano: 2026, mes: 10 };
      expect(previsaoPorMes(f, hoje)).toEqual({ meses: 5, data: '2027-02-01' });
      expect(dataAlvoPeloPorMes(f, 3000, hoje)).toBe('2027-02-01');
      // Acima do limite, sem valor por mês, ou sem nada a guardar: a data não muda.
      expect(dataAlvoPeloPorMes(f, 500, hoje)).toBeNull();
      expect(dataAlvoPeloPorMes({ ...f, porMes: null }, 3000, hoje)).toBeNull();
      expect(dataAlvoPeloPorMes({ ...f, dados: { ...f.dados, valorAtual: 6000 } }, 3000, hoje)).toBeNull();
    });
  });

  describe('"Como está indo"', () => {
    it('hojeDaEvolucao usa o mês do plano, se houver, e o dia do relógio', () => {
      const agora = new Date(2026, 10, 5, 23, 30);
      expect(hojeDaEvolucao({ ano: 2026, mes: 10 }, agora)).toEqual({ hoje: { ano: 2026, mes: 10 }, hojeDia: '2026-11-05' });
      expect(hojeDaEvolucao(null, agora)).toEqual({ hoje: { ano: 2026, mes: 11 }, hojeDia: '2026-11-05' });
    });

    it('graficoDaEvolucao: guardado, plano (só se existe) e alvo, na cor da meta', () => {
      const ev: EvolucaoDaMeta = { escala: 'mes', rotulos: ['out/26', 'nov/26'], guardado: [100, 200], plano: [null, 300], alvo: 500, aportes: [] };
      const nomes = (g: ReturnType<typeof graficoDaEvolucao>) => g.data.datasets.map((d) => d.label);
      const g = graficoDaEvolucao('#92c8f0', ev);
      expect(nomes(g)).toEqual(['Guardado', 'Plano', 'Alvo']);
      expect(g.data.datasets[0].borderColor).toBe('#92c8f0');
      expect(g.data.datasets[2].data).toEqual([500, 500]);
      expect(nomes(graficoDaEvolucao('#92c8f0', { ...ev, plano: [null, null] }))).toEqual(['Guardado', 'Alvo']);
    });
  });
});
