import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { ApiService } from '../../src/app/core/api.service';
import { RenomearEmLote, Transacao } from '../../src/app/core/modelos';
import { RenomearEmLoteComponent } from '../../src/app/paginas/renomear-em-lote.component';
import { transacao } from '../apoio/transacoes-edicao';

/** Abre "Renomear N transações" com a API falsa; guarda cada pedido. */
async function abrir(lista: Transacao[]) {
  const pedidos: RenomearEmLote[] = [];
  const api = {
    renomearEmLote: (dto: RenomearEmLote) => {
      pedidos.push(dto);
      return of({ alteradas: dto.ids.length + 2, regrasCriadas: dto.criarRegras ? 2 : 0, regrasAlteradas: 0, desfazer: 'lote-1' });
    },
    desfazerRenomear: () => of({ transacoesAtualizadas: 1 })
  };
  TestBed.configureTestingModule({ providers: [{ provide: ApiService, useValue: api }] });
  const fixture = TestBed.createComponent(RenomearEmLoteComponent);
  let renomeada = 0;
  fixture.componentInstance.renomeada.subscribe(() => renomeada++);
  fixture.componentRef.setInput('transacoes', lista);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { fixture, tela: fixture.componentInstance, raiz: fixture.nativeElement as HTMLElement, pedidos, renomeada: () => renomeada };
}

/** Regressão do renomear em massa (#123): nome nas selecionadas, com regras "é igual a" para as próximas. */
describe('Regressão: renomear transações em massa', () => {
  const doBanco = [
    transacao({ id: 1, descricao: 'Pagamento recebido', descricaoOriginal: 'Pagamento recebido' }),
    transacao({ id: 2, descricao: 'Pagamento recebido', descricaoOriginal: 'PAGAMENTO RECEBIDO' }),
    transacao({ id: 3, descricao: 'Pagamento Cartão de crédito', descricaoOriginal: 'Pagamento Cartão de crédito' })
  ];

  it('vindo do banco, "nas próximas" já vem marcado e conta uma regra por descrição do banco (sem diferença de maiúsculas)', async () => {
    const t = await abrir(doBanco);
    expect(t.tela.escopo).toBe('regras');
    expect(t.tela.originais()).toEqual(['Pagamento recebido', 'Pagamento Cartão de crédito']);
    expect(t.raiz.textContent).toContain('cria 2 regras “é igual a”');
  });

  it('manda os ids, o nome e criarRegras; o aviso tem "Desfazer"', async () => {
    const t = await abrir(doBanco);
    t.tela.nome = ' Pagamento da fatura ';
    t.tela.salvar();
    expect(t.pedidos).toEqual([{ ids: [1, 2, 3], descricao: 'Pagamento da fatura', criarRegras: true }]);
    expect(t.renomeada()).toBe(1);
    const aviso = TestBed.inject(AvisosService).avisos().find((a) => a.acao)!;
    expect(aviso.texto).toBe('5 transação(ões) renomeada(s), 2 regra(s) criada(s).');
  });

  it('"só nas selecionadas" não cria regra', async () => {
    const t = await abrir(doBanco);
    t.tela.escopo = 'so-estas';
    t.tela.nome = 'Fatura';
    t.tela.salvar();
    expect(t.pedidos[0].criarRegras).toBe(false);
  });

  it('lançamentos manuais (sem descrição do banco): a opção de regra fica desligada', async () => {
    const t = await abrir([transacao({ id: 9, descricaoOriginal: null, openFinance: false })]);
    expect(t.tela.escopo).toBe('so-estas');
    expect(t.raiz.querySelector<HTMLInputElement>('input[value=regras]')!.disabled).toBe(true);
  });
});
