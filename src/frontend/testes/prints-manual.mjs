// #752, ADR 0071: `npm run prints:manual`. Compila o site e regera os prints do Manual (public/imagens/manual/ e
// src/app/paginas/manual/prints-manual.ts) pelo testes/navegador/prints-manual.spec.ts, com a API falsa e o Chromium do
// Playwright (o mesmo da CI: `npx playwright install chromium` uma vez). Rode antes de cada versão estável e envie o
// que mudou no commit. Aceita um filtro, ex.: npm run prints:manual -- painel (só os ids que contêm "painel").
import { spawnSync } from 'node:child_process';

const env = { ...process.env, PRINTS_MANUAL: '1', NAVEGADOR: 'chromium' };
const shell = process.platform === 'win32';
const filtro = process.argv.slice(2);

for (const comando of [
  ['npm', 'run', 'build'],
  ['npx', 'playwright', 'test', 'prints-manual', '--project=computador', ...filtro.flatMap((f) => ['-g', f])]
]) {
  const r = spawnSync(comando[0], comando.slice(1), { stdio: 'inherit', env, shell });
  if (r.status !== 0) process.exit(r.status ?? 1);
}
