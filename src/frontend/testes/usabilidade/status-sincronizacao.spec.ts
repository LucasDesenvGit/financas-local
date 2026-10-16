import { computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { StatusSincronizacao } from '../../src/app/core/modelos';
import { StatusSincronizacaoService } from '../../src/app/core/status-sincronizacao.service';
import { StatusSincronizacaoComponent } from '../../src/app/compartilhado/status-sincronizacao.component';

/** Serviço falso: os testes mudam os sinais e conferem o que a barra mostra e anuncia. */
function servicoFalso() {
  const agora = new Date(2026, 9, 16, 15, 0);
  const status = signal<StatusSincronizacao | null>({
    ultimaSincronizacao: new Date(2026, 9, 16, 13, 0).toISOString(), emExecucao: false,
    conexoesAtivas: 2, conexoesComErro: 0, consentimentosVencendo: 0
  });
  const sincronizando = signal(false);
  return {
    status, sincronizando,
    agora: signal(agora),
    ultima: computed(() => (status()?.ultimaSincronizacao ? new Date(status()!.ultimaSincronizacao!) : null)),
    conexoesComErro: computed(() => status()?.conexoesComErro ?? 0),
    consentimentosVencendo: computed(() => status()?.consentimentosVencendo ?? 0),
    sincronizar: jest.fn(), iniciar: jest.fn(), parar: jest.fn()
  };
}

/**
 * Usabilidade do status da sincronização na barra superior (UX-8, #38). Nielsen 1 (visibilidade do estado do
 * sistema): dá para saber se os dados estão atualizados sem abrir Configurações; mudanças são anunciadas
 * (aria-live) e o botão só com ícone tem nome acessível (WCAG 4.1.2).
 */
describe('Usabilidade: status da sincronização', () => {
  function montar() {
    const servico = servicoFalso();
    TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: StatusSincronizacaoService, useValue: servico }] });
    const fixture = TestBed.createComponent(StatusSincronizacaoComponent);
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    return {
      fixture, raiz, servico,
      botao: () => raiz.querySelector<HTMLButtonElement>('.botao-sincronizar')!,
      anuncio: () => raiz.querySelector('[aria-live="polite"]')!.textContent!.trim()
    };
  }

  it('mostra há quanto tempo foi a última sincronização, com data completa no nome acessível', () => {
    const { botao } = montar();
    expect(botao().textContent).toContain('Atualizado há 2 h');
    expect(botao().getAttribute('aria-label')).toMatch(/^Sincronizar bancos\. Última sincronização há 2 h, em 16\/10\/2026 \d{2}:00\.$/);
  });

  it('durante a sincronização: botão desabilitado, rótulo muda e o leitor de tela é avisado do início e do fim', () => {
    const { fixture, servico, botao, anuncio } = montar();
    expect(anuncio()).toBe('');                          // nada é anunciado ao abrir a tela
    servico.sincronizando.set(true);
    fixture.detectChanges();
    expect(botao().disabled).toBe(true);
    expect(botao().textContent).toContain('Sincronizando…');
    expect(anuncio()).toBe('Sincronizando os bancos…');
    servico.sincronizando.set(false);
    fixture.detectChanges();
    expect(anuncio()).toBe('Sincronização concluída.');
  });

  it('conexão com erro vira um link para as conexões, com o problema no nome', () => {
    const { fixture, servico, raiz, anuncio } = montar();
    servico.status.update((s) => ({ ...s!, conexoesComErro: 1, consentimentosVencendo: 2 }));
    fixture.detectChanges();
    const alerta = raiz.querySelector<HTMLAnchorElement>('a.alerta-sincronizacao')!;
    expect(alerta.getAttribute('href')).toBe('/configuracoes');
    expect(alerta.getAttribute('aria-label')).toBe('1 conexão com erro: ver conexões');   // erro tem prioridade
    expect(anuncio()).toBe('1 conexão com erro');
  });

  it('sem problema, não mostra alerta (Nielsen 8)', () => {
    const { raiz } = montar();
    expect(raiz.querySelector('.alerta-sincronizacao')).toBeNull();
  });
});
