import { Recorrencia } from '../../src/app/core/modelos';
import { avancarVencimento } from '../../src/app/paginas/contas-agendadas.component';
import { abrirContasAPagar, contaAgendada } from '../apoio/contas-agendadas';

/** Regressão da data limite de Contas a pagar (#69): lista, último pagamento, encerrada e o que vai para a API. */
describe('Regressão: data limite de conta a pagar', () => {
  const textoLinha = (raiz: HTMLElement, i: number) =>
    raiz.querySelectorAll('table')[0].querySelectorAll('tbody tr')[i].textContent!.replace(/\s+/g, ' ');

  it('conta com data limite mostra até quando vai e quantos pagamentos faltam', async () => {
    const t = abrirContasAPagar([], [
      contaAgendada({ id: 1, dataLimite: '2026-12-10', pagamentosRestantes: 2 }),
      contaAgendada({ id: 2, descricao: 'Curso', dataLimite: '2026-11-10', pagamentosRestantes: 1 })
    ]);
    await t.atualizar();
    expect(textoLinha(t.raiz, 0)).toMatch(/Mensal até dez\.?\/2026 · faltam 2/);
    expect(textoLinha(t.raiz, 1)).toMatch(/Mensal até nov\.?\/2026 · último pagamento/);
  });

  it('encerrada fica identificada, sem Pagar, e fora dos totais', async () => {
    const t = abrirContasAPagar([], [
      contaAgendada({ ativa: false, encerrada: true, dataLimite: '2026-09-10', proximoVencimento: '2026-10-10',
        ultimoPagamentoEm: '2026-09-08', diasParaVencer: -6 })
    ]);
    await t.atualizar();
    const linha = textoLinha(t.raiz, 0);
    expect(linha).toContain('encerrada em 08/09/2026');
    expect(linha).not.toContain('10/10/2026');
    expect(t.raiz.querySelector('tbody tr button.botao-pequeno')).toBeNull();
    expect(t.raiz.querySelector('.kpi strong')!.textContent).toMatch(/R\$\s0,00/);
  });

  it('pagar o último pagamento avisa que a conta será encerrada', async () => {
    const t = abrirContasAPagar([], [contaAgendada({ dataLimite: '2026-11-10', pagamentosRestantes: 1 })]);
    await t.atualizar();
    t.tela.abrirPagamento(t.tela.itens()[0]);
    await t.atualizar();
    expect(t.raiz.querySelector('[role=status]')!.textContent).toContain('É o último pagamento (até 10/11/2026): a conta será encerrada.');
  });

  it('editar mantém a data limite; mudar para única manda sem data limite', async () => {
    const t = abrirContasAPagar([], [contaAgendada({ id: 7, dataLimite: '2027-06-10', pagamentosRestantes: 8 })]);
    await t.atualizar();
    t.tela.editar(t.tela.itens()[0]);
    t.tela.salvar();
    t.tela.editar(t.tela.itens()[0]);
    t.tela.form!.recorrencia = Recorrencia.Unica;
    t.tela.salvar();
    expect(t.salvos.map((s) => s.dataLimite)).toEqual(['2027-06-10', null]);
  });

  it('avança o vencimento como a API: dia que não existe cai no último dia do mês', () => {
    expect(avancarVencimento('2027-01-31', Recorrencia.Mensal)).toBe('2027-02-28');
    expect(avancarVencimento('2027-02-28', Recorrencia.Mensal)).toBe('2027-03-28');
    expect(avancarVencimento('2028-02-29', Recorrencia.Anual)).toBe('2029-02-28');
    expect(avancarVencimento('2026-12-10', Recorrencia.Mensal)).toBe('2027-01-10');
  });
});
