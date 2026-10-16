/**
 * O jsdom não implementa a Popover API (showPopover/hidePopover e o seletor :popover-open), usada pelo balão do
 * glossário (TermoComponent). Este substituto só guarda o estado num atributo: o que se testa é o comportamento do
 * componente (quando abre, onde posiciona, como fecha), não o desenho na top layer, que fica com o navegador.
 */
export function instalarPopoverFalso(): void {
  const prototipo = HTMLElement.prototype as HTMLElement & Record<string, unknown>;
  if (typeof prototipo.showPopover === 'function') return;
  prototipo.showPopover = function (this: HTMLElement) { this.setAttribute('data-popover-aberto', ''); };
  prototipo.hidePopover = function (this: HTMLElement) { this.removeAttribute('data-popover-aberto'); };
  const matchesOriginal = Element.prototype.matches;
  Element.prototype.matches = function (this: Element, seletor: string): boolean {
    if (seletor === ':popover-open') return this.hasAttribute('data-popover-aberto');
    return matchesOriginal.call(this, seletor);
  };
}
