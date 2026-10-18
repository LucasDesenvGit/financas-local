import { DOCUMENT } from '@angular/common';
import { Directive, ElementRef, NgZone, OnDestroy, inject } from '@angular/core';

let proximoId = 0;

/**
 * Lista suspensa no visual do site, por cima do <select> nativo (que continua dono do valor: ngModel, [ngValue],
 * (change) e formulários não mudam). No computador, clicar ou abrir pelo teclado (Espaço, Enter, Alt+↓, F4) mostra
 * a lista do site: opção escolhida destacada, ↑/↓/Home/End e letra para andar, Enter para escolher, Esc para fechar,
 * com animação de entrada e saída. No toque (pointer: coarse) fica a lista do sistema, que é a melhor no celular.
 * A lista é montada com textContent (nada de innerHTML) e vai para o <body>, para não ser cortada por um modal ou
 * uma tabela com rolagem.
 * Uso: importar a diretiva no componente; o seletor pega todo <select> sem `multiple`.
 */
@Directive({
  selector: 'select:not([multiple])',
  host: {
    '(mousedown)': 'aoPressionar($event)',
    '(keydown)': 'aoTeclar($event)',
    '(blur)': 'fechar()'
  }
})
export class ListaSuspensaDirective implements OnDestroy {
  private readonly select = inject<ElementRef<HTMLSelectElement>>(ElementRef).nativeElement;
  private readonly documento = inject(DOCUMENT);
  private readonly zona = inject(NgZone);
  private readonly id = `lista-suspensa-${++proximoId}`;
  private painel: HTMLElement | null = null;
  private ativa = -1;
  private readonly aoClicarFora = (e: Event) => {
    if (!this.painel?.contains(e.target as Node) && e.target !== this.select) this.fechar();
  };
  private readonly aoRolarOuRedimensionar = (e: Event) => {
    if (!this.painel?.contains(e.target as Node)) this.fechar();
  };

  get aberta(): boolean {
    return this.painel !== null && !this.painel.classList.contains('sair');
  }

  aoPressionar(e: MouseEvent): void {
    if (e.button !== 0 || this.select.disabled || this.toque()) return;
    e.preventDefault();          // não abre a lista do sistema
    this.select.focus();
    if (this.aberta) this.fechar(); else this.abrir();
  }

  aoTeclar(e: KeyboardEvent): void {
    if (this.select.disabled) return;
    if (!this.aberta) {
      const abre = e.key === ' ' || e.key === 'Enter' || e.key === 'F4' || (e.altKey && (e.key === 'ArrowDown' || e.key === 'ArrowUp'));
      if (abre && !this.toque()) {
        e.preventDefault();
        this.abrir();
      }
      return;                    // ↑/↓ com a lista fechada: o select nativo troca o valor, como sempre
    }
    const opcoes = this.select.options;
    switch (e.key) {
      case 'ArrowDown': this.mover(this.proxima(this.ativa, 1)); break;
      case 'ArrowUp': this.mover(this.proxima(this.ativa, -1)); break;
      case 'Home': this.mover(this.proxima(-1, 1)); break;
      case 'End': this.mover(this.proxima(opcoes.length, -1)); break;
      case 'Enter': case ' ': this.escolher(this.ativa); break;
      case 'Escape': this.fechar(); e.stopPropagation(); break;   // não fecha o modal em volta
      case 'Tab': this.fechar(); return;
      default:
        if (e.key.length === 1) { this.porLetra(e.key); break; }
        return;
    }
    e.preventDefault();
  }

  abrir(): void {
    if (this.painel) this.removerPainel();
    const painel = this.documento.createElement('div');
    painel.id = this.id;
    painel.className = 'lista-suspensa-painel';
    painel.setAttribute('role', 'listbox');
    const rotulo = this.rotulo();
    if (rotulo) painel.setAttribute('aria-label', rotulo);
    Array.from(this.select.options).forEach((opcao, i) => {
      const item = this.documento.createElement('div');
      item.id = `${this.id}-${i}`;
      item.className = 'lista-suspensa-opcao';
      item.setAttribute('role', 'option');
      item.setAttribute('aria-selected', String(opcao.selected));
      if (opcao.disabled) item.setAttribute('aria-disabled', 'true');
      item.textContent = opcao.textContent;
      // mousedown sem foco: o foco fica no select (senão o blur fecharia a lista antes do clique)
      item.addEventListener('mousedown', (ev) => ev.preventDefault());
      item.addEventListener('click', () => this.escolher(i));
      item.addEventListener('mousemove', () => this.mover(i, false));
      painel.appendChild(item);
    });
    this.documento.body.appendChild(painel);
    this.painel = painel;
    this.posicionar();
    this.select.setAttribute('aria-expanded', 'true');
    this.select.setAttribute('aria-controls', this.id);
    this.mover(this.select.selectedIndex >= 0 ? this.select.selectedIndex : this.proxima(-1, 1));
    this.zona.runOutsideAngular(() => {
      this.documento.addEventListener('mousedown', this.aoClicarFora, true);
      this.documento.defaultView?.addEventListener('scroll', this.aoRolarOuRedimensionar, true);
      this.documento.defaultView?.addEventListener('resize', this.aoRolarOuRedimensionar);
    });
  }

  fechar(): void {
    if (!this.painel || this.painel.classList.contains('sair')) return;
    this.pararDeOuvir();
    this.select.removeAttribute('aria-expanded');
    this.select.removeAttribute('aria-activedescendant');
    const painel = this.painel;
    painel.classList.add('sair');
    // Espera a animação de saída; sem animação (reduzir movimento, ou ambiente sem CSS), sai na hora.
    const animacao = this.documento.defaultView?.getComputedStyle(painel).animationName;
    if (!animacao || animacao === 'none') { this.removerPainel(); return; }
    const remover = () => { if (this.painel === painel) this.removerPainel(); };
    painel.addEventListener('animationend', remover, { once: true });
    setTimeout(remover, 250);
  }

  ngOnDestroy(): void {
    this.pararDeOuvir();
    this.removerPainel();
  }

  /* ------------------------------ interno ------------------------------ */

  private escolher(i: number): void {
    const opcao = this.select.options[i];
    if (!opcao || opcao.disabled) return;
    const mudou = this.select.selectedIndex !== i;
    this.select.selectedIndex = i;
    this.fechar();
    // O SelectControlValueAccessor (ngModel/[ngValue]) e os (change) das telas ouvem o evento nativo.
    if (mudou) this.select.dispatchEvent(new Event('change', { bubbles: true }));
  }

  private mover(i: number, rolar = true): void {
    if (!this.painel || i < 0) return;
    this.ativa = i;
    Array.from(this.painel.children).forEach((item, j) => item.classList.toggle('ativa', j === i));
    const item = this.painel.children[i] as HTMLElement | undefined;
    if (item) {
      this.select.setAttribute('aria-activedescendant', item.id);
      if (rolar) item.scrollIntoView?.({ block: 'nearest' });
    }
  }

  /** Próxima opção habilitada a partir de `de`, na direção `passo` (ou a atual, se não houver). */
  private proxima(de: number, passo: 1 | -1): number {
    const opcoes = this.select.options;
    for (let i = de + passo; i >= 0 && i < opcoes.length; i += passo) {
      if (!opcoes[i].disabled) return i;
    }
    return this.ativa;
  }

  private porLetra(letra: string): void {
    const opcoes = Array.from(this.select.options);
    const alvo = letra.toLocaleLowerCase('pt-BR');
    const ordem = [...opcoes.slice(this.ativa + 1), ...opcoes.slice(0, this.ativa + 1)];
    const achada = ordem.find((o) => !o.disabled && (o.textContent ?? '').trim().toLocaleLowerCase('pt-BR').startsWith(alvo));
    if (achada) this.mover(opcoes.indexOf(achada));
  }

  /** Abaixo do campo; sem espaço embaixo, acima. Pelo menos a largura do campo. */
  private posicionar(): void {
    if (!this.painel) return;
    const r = this.select.getBoundingClientRect();
    const janela = this.documento.defaultView;
    const alturaJanela = janela?.innerHeight ?? 800;
    const espacoAbaixo = alturaJanela - r.bottom;
    const altura = Math.min(this.painel.scrollHeight || 300, 300);
    this.painel.style.minWidth = `${r.width}px`;
    this.painel.style.left = `${Math.max(8, r.left)}px`;
    if (espacoAbaixo < altura + 12 && r.top > espacoAbaixo) {
      this.painel.style.bottom = `${alturaJanela - r.top + 4}px`;
      this.painel.classList.add('acima');
    } else {
      this.painel.style.top = `${r.bottom + 4}px`;
    }
  }

  /** Nome da lista: o aria-label do select ou o texto do <label> (sem o texto das opções, que está dentro dele). */
  private rotulo(): string | null {
    const proprio = this.select.getAttribute('aria-label');
    if (proprio) return proprio;
    const label = this.select.labels?.[0];
    if (!label) return null;
    const copia = label.cloneNode(true) as HTMLElement;
    copia.querySelectorAll('select, button, input').forEach((n) => n.remove());
    return copia.textContent?.replace(/\s+/g, ' ').trim() || null;
  }

  private toque(): boolean {
    return this.documento.defaultView?.matchMedia?.('(pointer: coarse)').matches ?? false;
  }

  private removerPainel(): void {
    this.painel?.remove();
    this.painel = null;
    this.ativa = -1;
    this.select.removeAttribute('aria-controls');
  }

  private pararDeOuvir(): void {
    this.documento.removeEventListener('mousedown', this.aoClicarFora, true);
    this.documento.defaultView?.removeEventListener('scroll', this.aoRolarOuRedimensionar, true);
    this.documento.defaultView?.removeEventListener('resize', this.aoRolarOuRedimensionar);
  }
}
