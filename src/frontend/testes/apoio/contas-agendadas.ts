import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { ContaAgendada, Recorrencia, SalvarContaAgendada, SugestaoContaAgendada } from '../../src/app/core/modelos';
import { ContasAgendadasComponent } from '../../src/app/paginas/contas-agendadas.component';

/** Sugestão de exemplo: escola paga todo dia 10, valor fixo. Os testes mudam só o que importa para cada caso. */
export function sugestao(campos: Partial<SugestaoContaAgendada> = {}): SugestaoContaAgendada {
  return {
    chave: 'escola fiap', descricao: 'BOLETO FIAP', valor: 1450, valorVariavel: false, valorMinimo: 1450, valorMaximo: 1450,
    proximoVencimento: '2026-11-10', diaHabitual: 10, meses: 6, pagamentos: 6,
    ultimoPagamento: '2026-10-10', primeiroPagamento: '2026-05-10',
    categoriaId: 5, categoriaNome: 'Educação', contaId: 3, contaNome: 'Itaú', termoBusca: 'FIAP',
    vencimentoSemPagamento: null, ...campos
  };
}

/** Conta a pagar de exemplo: escola mensal, ativa, sem data limite. */
export function contaAgendada(campos: Partial<ContaAgendada> = {}): ContaAgendada {
  return {
    id: 1, descricao: 'Escola', valor: 1450, proximoVencimento: '2026-11-10', recorrencia: Recorrencia.Mensal,
    categoriaId: null, categoriaNome: null, contaId: null, contaNome: null, ativa: true, ultimoPagamentoEm: null,
    diasParaVencer: 25, dataLimite: null, pagamentosRestantes: null, encerrada: false, ...campos
  };
}

/**
 * Abre Contas a pagar com a API falsa; `sugestoes` = o que GET /contas-agendadas/sugestoes devolve (ou um erro),
 * `itens` = a lista. `salvos` guarda cada corpo enviado ao criar/editar.
 */
export function abrirContasAPagar(sugestoes: SugestaoContaAgendada[] | 'erro', itens: ContaAgendada[] = []) {
  const salvos: SalvarContaAgendada[] = [];
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      {
        provide: ApiService,
        useValue: {
          contasAgendadas: () => of(itens),
          criarContaAgendada: (dto: SalvarContaAgendada) => { salvos.push(dto); return of(undefined); },
          atualizarContaAgendada: (_id: number, dto: SalvarContaAgendada) => { salvos.push(dto); return of(undefined); },
          contas: () => of([]),
          categorias: () => of([]),
          sugestoesContasAgendadas: () => (sugestoes === 'erro' ? throwError(() => new Error('falhou')) : of(sugestoes))
        }
      }
    ]
  });
  const fixture = TestBed.createComponent(ContasAgendadasComponent);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const linhas = () => Array.from(raiz.querySelectorAll('section.sugestoes tbody tr'));
  const botao = (rotulo: string) => raiz.querySelector<HTMLElement>(`[aria-label="${rotulo}"]`)!;
  const clicar = (el: HTMLElement) => { el.click(); fixture.detectChanges(); };
  const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  return { fixture, tela: fixture.componentInstance, raiz, linhas, botao, clicar, atualizar, salvos };
}
