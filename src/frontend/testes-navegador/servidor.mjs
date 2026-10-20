// Servidor dos testes no navegador (#185, ADR 0041): serve o build de produção (dist/) como a API serve o wwwroot,
// com a MESMA Content-Security-Policy, lida do Seguranca.cs da API (não há cópia para ficar velha). Assim um script
// inline bloqueado, como o do #157, aparece no teste. As chamadas /api/** nunca chegam aqui: o Playwright as responde
// com a API falsa (api-falsa.ts).
//
// Uso: node testes-navegador/servidor.mjs [porta]   (padrão 4300)
import { createReadStream, existsSync, readFileSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, normalize, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = fileURLToPath(new URL('.', import.meta.url));
const pasta = resolve(aqui, '..', 'dist', 'financas-web', 'browser');
const porta = Number(process.argv[2] ?? process.env.PORTA ?? 4300);

if (!existsSync(join(pasta, 'index.html'))) {
  console.error(`Build de produção não encontrado em ${pasta}. Rode antes: npm run build`);
  process.exit(1);
}

/** A CSP da API: as partes de "private const string Csp = \"...\" + \"...\";" do Seguranca.cs, juntas. */
function cspDaApi() {
  const arquivo = resolve(aqui, '..', '..', 'backend', 'Financas.Api', 'Seguranca', 'Seguranca.cs');
  const fonte = readFileSync(arquivo, 'utf8');
  // As partes entre aspas, ligadas por "+", até o ";" que fecha a instrução. A CSP tem ";" DENTRO das aspas
  // ('self';), então não dá para parar no primeiro ";".
  const bloco = fonte.match(/const string Csp\s*=\s*((?:"[^"]*"\s*\+?\s*)+);/);
  if (!bloco) throw new Error(`Não achei a constante Csp em ${arquivo}`);
  return [...bloco[1].matchAll(/"([^"]*)"/g)].map((m) => m[1]).join('').trim();
}

const csp = cspDaApi();
// Sem CSP, os testes passariam sem testar nada (aconteceu enquanto este servidor era escrito): melhor não subir.
if (!/script-src\s+'self'/.test(csp) || !/default-src/.test(csp)) {
  console.error(`A CSP lida do Seguranca.cs parece errada: "${csp}"`);
  process.exit(1);
}
const tipos = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json',
  '.svg': 'image/svg+xml', '.ico': 'image/x-icon', '.png': 'image/png', '.webmanifest': 'application/manifest+json',
  '.woff2': 'font/woff2'
};

createServer((req, res) => {
  const caminho = decodeURIComponent(new URL(req.url ?? '/', 'http://x').pathname);
  if (caminho.startsWith('/api/')) {
    // Não deveria chegar: toda chamada da API é respondida pela API falsa do teste.
    res.writeHead(599, { 'Content-Type': 'text/plain' }).end(`chamada sem simulação: ${req.method} ${caminho}`);
    return;
  }
  let arquivo = normalize(join(pasta, caminho));
  if (!arquivo.startsWith(pasta) || !existsSync(arquivo) || statSync(arquivo).isDirectory()) arquivo = join(pasta, 'index.html');
  res.writeHead(200, {
    'Content-Type': tipos[extname(arquivo)] ?? 'application/octet-stream',
    'Content-Security-Policy': csp,
    'Cache-Control': 'no-store'
  });
  createReadStream(arquivo).pipe(res);
}).listen(porta, '127.0.0.1', () => console.log(`Servindo ${pasta} em http://127.0.0.1:${porta} (CSP da API)`));
