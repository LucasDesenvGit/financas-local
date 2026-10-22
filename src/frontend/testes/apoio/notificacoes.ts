import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import { NotificacoesComponent } from '../../src/app/compartilhado/notificacoes.component';
import { ApiService } from '../../src/app/core/api.service';
import { Avisos } from '../../src/app/core/modelos';

/**
 * #327: o sino das notificações (barra do topo) com a API falsa (só GET /painel/avisos, mês atual). `abrir` já clica
 * no sino; `respostas` guarda cada busca. O retorno tem `harness.fixture` como o `abrirPainel`, para destruir e abrir de
 * novo (simula recarregar a tela).
 */
export async function abrirNotificacoes(dados: Partial<Avisos> = {}, abrir = true, falhar = false) {
  const hoje = new Date();
  const avisos: Avisos = { ano: hoje.getFullYear(), mes: hoje.getMonth() + 1, alertas: [], orcamentosEstourados: [], ...dados };
  let buscas = 0;
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: ApiService, useValue: { avisos: () => { buscas++; return falhar ? throwError(() => new Error('404')) : of(avisos); } } }]
  });
  const fixture = TestBed.createComponent(NotificacoesComponent);
  fixture.detectChanges();
  await fixture.whenStable();
  const raiz = fixture.nativeElement as HTMLElement;
  const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  const sino = () => raiz.querySelector<HTMLButtonElement>('.botao-notificacoes')!;
  if (abrir) {
    sino().click();
    await atualizar();
  }
  return { raiz, atualizar, sino, tela: fixture.componentInstance, harness: { fixture }, buscas: () => buscas };
}
