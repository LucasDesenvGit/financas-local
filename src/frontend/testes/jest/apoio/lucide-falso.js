// Substituto do @lucide/angular nos testes do Jest (#204). O pacote de verdade é um arquivo de 10,6 MB com ~1.900
// componentes, e o Angular em JIT compila todos ao carregá-lo: ~15 s em CADA arquivo de teste que encosta num ícone.
// Aqui fica só o que o site usa:
// - a diretiva [lucideIcon] (mesmas entradas), sem desenhar nada dentro do <svg> (o <svg> e o aria-hidden são do nosso
//   template, então os testes continuam conferindo);
// - cada ícone (LucideEye, LucideEyeOff…) vira um objeto fixo com o nome, o mesmo a cada acesso: dá para conferir QUAL
//   ícone a tela usa (ex.: Icones.mostrar × Icones.ocultar, #135).
// O build e os testes no navegador (Playwright) usam o pacote de verdade.
const { Directive } = require('@angular/core');

class LucideDynamicIcon {}
Directive({ selector: '[lucideIcon]', inputs: ['lucideIcon', 'size', 'strokeWidth', 'color', 'absoluteStrokeWidth'] })(LucideDynamicIcon);

const icones = new Map();
const base = { __esModule: true, LucideDynamicIcon };

module.exports = new Proxy(base, {
  get(alvo, nome) {
    if (nome in alvo) return alvo[nome];
    if (typeof nome === 'string' && /^Lucide[A-Z]/.test(nome)) {
      if (!icones.has(nome)) icones.set(nome, Object.freeze({ nome }));
      return icones.get(nome);
    }
    return undefined;
  }
});
