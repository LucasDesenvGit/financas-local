import { TestBed } from '@angular/core/testing';
import { ConfirmacaoComponent } from '../../../src/app/compartilhado/confirmacao.component';
import { ConfirmacaoService } from '../../../src/app/compartilhado/confirmacao.service';

/**
 * #631 (ADR 0057): a confirmação do site, no lugar do `confirm()` do navegador.
 * - WAI-ARIA, "Alert and Message Dialogs": `role="alertdialog"`, a pergunta em `aria-describedby` e o foco no Cancelar;
 * - Nielsen 5: o botão diz o verbo da ação e sai vermelho quando apaga; Nielsen 3: Cancelar, Esc e "×" não fazem nada.
 */
describe('Usabilidade: confirmação do site no lugar do confirm() (#631)', () => {
  function abrir() {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(ConfirmacaoComponent);
    document.body.appendChild(fixture.nativeElement);
    fixture.detectChanges();
    const servico = TestBed.inject(ConfirmacaoService);
    const raiz = fixture.nativeElement as HTMLElement;
    const pedir = (pedido: Parameters<ConfirmacaoService['confirmar']>[0]) => {
      const resposta = servico.confirmar(pedido);
      fixture.detectChanges();
      return resposta;
    };
    const janela = () => raiz.querySelector<HTMLElement>('section.modal');
    const botao = (texto: string) =>
      Array.from(raiz.querySelectorAll<HTMLButtonElement>('section.modal button')).find((b) => b.textContent!.trim() === texto)!;
    return { fixture, servico, raiz, pedir, janela, botao };
  }

  afterEach(() => document.body.innerHTML = '');

  it('abre como alertdialog, com o título, a pergunta ligada por aria-describedby e o foco no Cancelar', () => {
    const t = abrir();
    void t.pedir({ titulo: 'Excluir meta', mensagem: 'Excluir a meta "Viagem"?', confirmar: 'Excluir', perigo: true });
    const janela = t.janela()!;
    expect(janela.getAttribute('role')).toBe('alertdialog');
    expect(janela.getAttribute('aria-modal')).toBe('true');
    expect(janela.getAttribute('aria-label')).toBe('Excluir meta');
    const descricao = document.getElementById(janela.getAttribute('aria-describedby')!);
    expect(descricao?.textContent).toBe('Excluir a meta "Viagem"?');
    expect(document.activeElement).toBe(t.botao('Cancelar'));
  });

  it('o botão de confirmar diz o verbo da ação e sai vermelho só quando a ação apaga', () => {
    const t = abrir();
    void t.pedir({ mensagem: 'Excluir?', confirmar: 'Excluir', perigo: true });
    expect(t.botao('Excluir').classList).toContain('botao-perigo');
    t.botao('Cancelar').click();
    t.fixture.detectChanges();
    void t.pedir({ mensagem: 'Aplicar 3 sugestões?', confirmar: 'Aplicar' });
    expect(t.botao('Aplicar').classList).toContain('botao-primario');
    expect(t.botao('Aplicar').classList).not.toContain('botao-perigo');
  });

  it('sem título nem verbo, usa "Confirmar"', () => {
    const t = abrir();
    void t.pedir('Continuar?');
    expect(t.janela()!.getAttribute('aria-label')).toBe('Confirmar');
    expect(t.botao('Confirmar')).toBeDefined();
  });

  it('confirmar responde true e fecha; Cancelar, Esc e "×" respondem false e fecham', async () => {
    const t = abrir();
    const sim = t.pedir({ mensagem: 'Excluir?', confirmar: 'Excluir' });
    t.botao('Excluir').click();
    t.fixture.detectChanges();
    await expect(sim).resolves.toBe(true);
    expect(t.janela()).toBeNull();

    const cancelar = t.pedir('Excluir?');
    t.botao('Cancelar').click();
    t.fixture.detectChanges();
    await expect(cancelar).resolves.toBe(false);
    expect(t.janela()).toBeNull();

    const esc = t.pedir('Excluir?');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    t.fixture.detectChanges();
    await expect(esc).resolves.toBe(false);

    const fechar = t.pedir('Excluir?');
    t.raiz.querySelector<HTMLButtonElement>('button[aria-label="Fechar"]')!.click();
    t.fixture.detectChanges();
    await expect(fechar).resolves.toBe(false);
  });

  it('um pedido novo com outro aberto responde false ao anterior: uma confirmação por vez', async () => {
    const t = abrir();
    const primeiro = t.pedir('Primeira?');
    const segundo = t.pedir({ mensagem: 'Segunda?', confirmar: 'Sim' });
    await expect(primeiro).resolves.toBe(false);
    expect(t.raiz.querySelector('#confirmacao-mensagem')!.textContent).toBe('Segunda?');
    t.botao('Sim').click();
    await expect(segundo).resolves.toBe(true);
  });
});
