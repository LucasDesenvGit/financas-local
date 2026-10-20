import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import {
  CompraSemOperacao, ConferenciaInvestimentos, CorrigirInvestimentos, CreditoDaCorretora, RendimentoComoInvestimento,
  TaxaB3, TipoAtivo
} from '../../src/app/core/modelos';
import { ConferirInvestimentosComponent } from '../../src/app/paginas/conferir-investimentos.component';

/** Débito de compra sem operação (o de R$ 592,75 de 15/06, com R$ 3,60 de troco). */
export function debito(campos: Partial<CompraSemOperacao> = {}): CompraSemOperacao {
  return { transacaoId: 1177, data: '2026-06-15', valor: -592.75, troco: 3.6, conta: 'Nubank',
    descricao: 'Compra de Renda Variável', ...campos };
}

export function troco(campos: Partial<CreditoDaCorretora> = {}): CreditoDaCorretora {
  return { transacaoId: 1161, data: '2026-06-15', valor: 3.6, descricao: 'Valor recebido de Investimentos', conta: 'Nubank',
    categoria: 'Rendimentos', compraData: '2026-06-15', compraValor: -592.75, vendaTicker: null, vendaQuantidade: null,
    vendaData: null, ...campos };
}

export function rendimento(campos: Partial<RendimentoComoInvestimento> = {}): RendimentoComoInvestimento {
  return { transacaoId: 1178, data: '2026-06-15', valor: 5.5, descricao: 'Valor recebido de Investimentos', conta: 'Nubank',
    categoria: 'Investimentos', categoriaAutomatica: 'Rendimentos', ...campos };
}

/** #173 a): um dia de duas compras (valores fictícios): saiu R$ 1.049,23, voltaram R$ 20,90 de troco e as cotas custaram R$ 1.028,10. */
export function taxa(campos: Partial<TaxaB3> = {}): TaxaB3 {
  return { data: '2026-09-14', conta: 'Nubank', debitado: 1049.23, trocos: 20.9, valorDasCotas: 1028.1, taxa: 0.23,
    operacoes: [{ operacaoId: 31, ticker: 'MXRF11', quantidade: 50, taxa: 0.12 }, { operacaoId: 32, ticker: 'XPLG11', quantidade: 5, taxa: 0.11 }],
    ...campos };
}

/** O caso do GARE11: cadastrado como ação, 72 cotas faltando (compra de 15/06 fora do histórico), troco e provento. */
export function conferencia(campos: Partial<ConferenciaInvestimentos> = {}): ConferenciaInvestimentos {
  return {
    tipos: [{ ativoId: 10, ticker: 'GARE11', nome: 'Guardian Logistica Fundo De Investimento Imobiliario', tipoAtual: TipoAtivo.Acao,
      motivo: 'o nome do fundo é “Guardian Logistica Fundo De Investimento Imobiliario”' }],
    cotas: [{ ativoId: 10, ticker: 'GARE11', naCorretora: 224, naCarteira: 152, posicaoEm: '2026-10-17T19:47:24',
      debitoSugeridoId: 1177, precoSugerido: 8.1826, fechamentoNoDia: 8.19 }],
    comprasSemOperacao: [debito(), debito({ transacaoId: 1176, valor: -558.96, troco: 0.78 })],
    trocos: [troco()],
    rendimentos: [rendimento()],
    categoriaDoTroco: { id: 7, nome: 'Investimentos' },
    ...campos
  };
}

/** Abre a conferência com a API falsa; guarda cada pedido de correção e de desfazer. */
export async function abrirConferencia(dados: ConferenciaInvestimentos = conferencia()) {
  const correcoes: CorrigirInvestimentos[] = [];
  const desfeitas: string[] = [];
  const api = {
    conferenciaInvestimentos: () => of(dados),
    corrigirInvestimentos: (dto: CorrigirInvestimentos) => {
      correcoes.push(dto);
      return of({ tipos: dto.ativosParaFii.length, compras: dto.compras.length, trocos: dto.trocos.length,
        rendimentos: dto.rendimentos.length, taxas: dto.taxas?.length ?? 0, desfazer: 'chave-1' });
    },
    desfazerCorrecaoInvestimentos: (chave: string) => { desfeitas.push(chave); return of({ transacoesAtualizadas: 3 }); }
  };
  TestBed.configureTestingModule({ providers: [{ provide: ApiService, useValue: api }] });
  const fixture = TestBed.createComponent(ConferirInvestimentosComponent);
  const eventos = { corrigido: 0, fechar: 0, lancarAMao: [] as { ticker: string; quantidade: number }[], verOperacoes: [] as string[] };
  fixture.componentInstance.corrigido.subscribe(() => eventos.corrigido++);
  fixture.componentInstance.fechar.subscribe(() => eventos.fechar++);
  fixture.componentInstance.lancarAMao.subscribe((e) => eventos.lancarAMao.push(e));
  fixture.componentInstance.verOperacoes.subscribe((t) => eventos.verOperacoes.push(t));
  fixture.componentRef.setInput('aberto', true);
  const assentar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  await assentar();
  const raiz = fixture.nativeElement as HTMLElement;
  const botao = (texto: string) => Array.from(raiz.querySelectorAll<HTMLButtonElement>('button'))
    .find((b) => b.textContent!.trim() === texto)!;
  return { fixture, tela: fixture.componentInstance, raiz, assentar, botao, correcoes, desfeitas, eventos };
}
