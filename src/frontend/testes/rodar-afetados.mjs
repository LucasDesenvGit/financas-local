// #286: `npm run test:afetados`. Roda localmente só os testes do front-end afetados pelo que mudou desde o develop,
// inclusive o que ainda não foi commitado, com a mesma decisão da CI (scripts/ci/testes-afetados.py, ADR 0047).
// Argumentos a mais vão para o script (ex.: npm run test:afetados -- --base origin/main).
// Acha o Python nos dois sistemas: no Windows, "python" (o "python3" pode ser só o atalho da Microsoft Store).
import { spawnSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const script = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', 'scripts', 'ci', 'testes-afetados.py');
const extras = process.argv.slice(2);
const argumentos = [script, ...(extras.includes('--base') ? [] : ['--base', 'origin/develop']), '--local', '--executar', ...extras];
const candidatos = process.platform === 'win32' ? ['python', 'py', 'python3'] : ['python3', 'python'];

for (const python of candidatos) {
  const versao = spawnSync(python, ['--version'], { encoding: 'utf8' });
  if (versao.status !== 0 || !/Python 3/.test(`${versao.stdout}${versao.stderr}`)) continue;
  const r = spawnSync(python, argumentos, { stdio: 'inherit' });
  process.exit(r.status ?? 1);
}
console.error('Python 3 não encontrado (tentei: ' + candidatos.join(', ') + '). Instale o Python 3 para rodar só os testes afetados.');
process.exit(1);
