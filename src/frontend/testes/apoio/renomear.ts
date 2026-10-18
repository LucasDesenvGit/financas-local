import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { PreviaRenomear, RenomearTransacao, TipoCorrespondencia, Transacao } from '../../src/app/core/modelos';
import { RenomearTransacaoComponent } from '../../src/app/paginas/renomear-transacao.component';
import { transacao } from './transacoes-edicao';

/** "PIX ENVIADO 12345" do banco, ainda com o nome do banco. */
export function pix(campos: Partial<Transacao> = {}): Transacao {
  return transacao({ id: 7, descricao: 'PIX ENVIADO 12345 JOAO', descricaoOriginal: 'PIX ENVIADO 12345 JOAO', ...campos });
}

/**
 * Abre a janela "Renomear" com a API falsa. `iguais`: quantas outras transações a prévia acha. Guarda cada pedido de
 * prévia, de renomear e de desfazer.
 */
export async function abrirRenomear(t: Transacao = pix(), iguais = 3) {
  const previas: { texto: string | null; tipo: TipoCorrespondencia }[] = [];
  const pedidos: RenomearTransacao[] = [];
  const desfeitos: string[] = [];
  const api = {
    previaRenomear: (_id: number, texto: string | null, tipo: TipoCorrespondencia) => {
      previas.push({ texto, tipo });
      const p: PreviaRenomear = {
        descricaoOriginal: t.descricaoOriginal, textoSugerido: 'PIX ENVIADO JOAO', quantidade: iguais,
        exemplos: iguais ? ['PIX ENVIADO 99887 JOAO'] : []
      };
      return of(p);
    },
    renomearTransacao: (_id: number, dto: RenomearTransacao) => {
      pedidos.push(dto);
      return of({ alteradas: dto.aplicarNasOutras ? iguais + 1 : 1, regraId: dto.aplicarNasOutras ? 5 : null, desfazer: 'chave-9' });
    },
    desfazerRenomear: (chave: string) => { desfeitos.push(chave); return of({ transacoesAtualizadas: 1 }); }
  };
  TestBed.configureTestingModule({ providers: [{ provide: ApiService, useValue: api }] });
  const fixture = TestBed.createComponent(RenomearTransacaoComponent);
  const eventos = { renomeada: 0, fechar: 0 };
  fixture.componentInstance.renomeada.subscribe(() => eventos.renomeada++);
  fixture.componentInstance.fechar.subscribe(() => eventos.fechar++);
  fixture.componentRef.setInput('transacao', t);
  const assentar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  await assentar();
  const raiz = fixture.nativeElement as HTMLElement;
  const botao = (texto: string) => Array.from(raiz.querySelectorAll<HTMLButtonElement>('button'))
    .find((b) => b.textContent!.trim() === texto)!;
  return { fixture, tela: fixture.componentInstance, raiz, assentar, botao, previas, pedidos, desfeitos, eventos };
}
