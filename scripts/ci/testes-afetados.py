#!/usr/bin/env python3
"""
#255 (ADR 0047): no PR, quais testes do front-end rodar, pelo que mudou em relação à base.

- Navegador (Playwright): pelas telas afetadas. Um arquivo de src/app afeta as telas (rotas de app.routes.ts) que o
  importam, direta ou indiretamente (grafo de imports do próprio código). Roda: os arquivos de teste que abrem essas
  telas (goto('/rota') / abrir(page, '/rota')), as linhas de telas.spec.ts dessas rotas, o celular.spec.ts e os
  arquivos de teste que mudaram.
- Jest: os testes ligados aos arquivos que mudaram (jest --findRelatedTests segue os imports) + os que leem o código
  como texto (readFileSync/readdirSync: o grafo do Jest não os enxerga) + os de teste que mudaram.
- Tudo: arquivo global (estilos, core/, layout/, rotas, index.html, configuração, apoio dos testes, a própria CI),
  arquivo do front-end que não dá para ligar a uma tela, ou sem base (push no develop/main). Na dúvida, tudo: errar
  para mais é seguro; para menos, não.
- Nada do front-end mudou: frontend=false (os jobs do front-end são pulados).

Uso na CI:  python3 scripts/ci/testes-afetados.py --base origin/develop   (escreve em $GITHUB_OUTPUT e no resumo)
Uso local:  python3 scripts/ci/testes-afetados.py --base origin/develop   (mostra o que rodaria e os comandos)
Rodar:      cd src/frontend && npm run test:afetados   (#286: --base origin/develop --local --executar; inclui o que
            ainda não foi commitado e roda Jest e navegador só nos afetados; NAVEGADOR=chromium para o navegador da CI)
Teste:      python3 scripts/ci/testes-afetados.py --arquivos a.ts b.css    (sem git; simula a lista de mudados)
"""
import argparse
import os
import re
import subprocess
import sys

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
FRONT = 'src/frontend'
APP = f'{FRONT}/src/app'

# Mudou um destes: roda tudo (afeta todas as telas ou o próprio jeito de testar).
GLOBAIS = [
    r'^src/frontend/src/styles\.css$', r'^src/frontend/src/index\.html$', r'^src/frontend/src/main\.ts$',
    r'^src/frontend/src/app/app\.[^/]+\.ts$', r'^src/frontend/src/app/app\.component\.ts$',
    r'^src/frontend/src/app/core/', r'^src/frontend/src/app/layout/',
    r'^src/frontend/(package(-lock)?\.json|angular\.json|tsconfig[^/]*\.json|jest\.config\.js|playwright\.config\.ts)$',
    r'^src/frontend/testes/(apoio|configurar-jest)', r'^src/frontend/testes-navegador/(base\.ts|api-falsa\.ts|servidor\.mjs)$',
    r'^src/frontend/public/', r'^src/frontend/src/assets/',
    # A CSP que os testes no navegador aplicam vem daqui (ADR 0041).
    r'^src/backend/Financas\.Api/Seguranca/',
    r'^\.github/workflows/ci\.yml$', r'^scripts/ci/testes-afetados\.py$',
]
# O que conta como "front-end" (fora disso, os jobs do front-end não rodam).
DO_FRONT = [r'^src/frontend/', r'^src/backend/Financas\.Api/Seguranca/', r'^\.github/workflows/ci\.yml$',
            r'^scripts/ci/testes-afetados\.py$']


def escapar(texto):
    """Escapa só o que é especial na regex do JavaScript (o --grep do Playwright)."""
    especiais = set('.*+?^${}()|[]\\/')
    return ''.join('\\' + c if c in especiais else c for c in texto)


def git(*args):
    return subprocess.run(['git', *args], cwd=RAIZ, capture_output=True, text=True, check=True).stdout


def mudados(base, local=False):
    """Arquivos mudados desde a base. Com local=True (#286), também o que ainda não foi commitado (alterado e novo)."""
    ponto = git('merge-base', base, 'HEAD').strip()
    lista = [l for l in git('diff', '--name-only', f'{ponto}...HEAD').splitlines() if l.strip()]
    if local:
        lista += git('diff', '--name-only', 'HEAD').splitlines()
        lista += git('ls-files', '--others', '--exclude-standard').splitlines()
    return sorted({l.strip() for l in lista if l.strip()})


def executar(d):
    """#286: roda localmente o que a CI rodaria (Jest e navegador só nos afetados). Devolve o código de saída."""
    import shutil
    front = os.path.join(RAIZ, FRONT)
    # Node direto com o executável de cada ferramenta, sem npx/npm: no Windows eles são .cmd e passam pelo cmd.exe,
    # que lê o "<" de "<rootDir>" como redirecionamento de arquivo.
    node = shutil.which('node') or 'node'
    jest = [node, os.path.join(front, 'node_modules', 'jest', 'bin', 'jest.js')]
    ng = [node, os.path.join(front, 'node_modules', '@angular', 'cli', 'bin', 'ng.js')]
    playwright = [node, os.path.join(front, 'node_modules', '@playwright', 'test', 'cli.js')]
    build = ('build de produção', [*ng, 'build', '--configuration', 'production'])
    passos = []
    if not d['frontend']:
        print('\nNada do front-end mudou: nenhum teste a rodar.')
        return 0
    if d['tudo']:
        passos += [('Jest (tudo)', jest), build, ('navegador (tudo)', [*playwright, 'test'])]
    else:
        if d.get('jest'):
            passos.append(('Jest (afetados)', [*jest, '--roots', '<rootDir>/testes', '--roots', '<rootDir>/src',
                                               '--findRelatedTests', *d['jest'], '--passWithNoTests']))
        if d.get('grep'):
            passos += [build, ('navegador (afetados)', [*playwright, 'test', '--grep', d['grep']])]
    if not passos:
        print('\nNenhum teste ligado ao que mudou.')
        return 0
    for nome, comando in passos:
        print(f'\n==> {nome}', flush=True)
        if subprocess.run(comando, cwd=front).returncode != 0:
            print(f'\nFalhou: {nome}.')
            return 1
    print('\nTudo certo nos testes afetados. A suíte inteira roda na CI, no merge.')
    return 0


def rotas_por_componente():
    """'src/frontend/src/app/paginas/x.component.ts' → ['rota', …] (a rota "manual/:capitulo" conta como "manual")."""
    texto = open(os.path.join(RAIZ, APP, 'app.routes.ts'), encoding='utf-8').read()
    mapa = {}
    for m in re.finditer(r"path:\s*'([^']*)'[^{}]*?import\('\./([^']+)'\)", texto, flags=re.S):
        rota = m.group(1).split('/')[0]
        arquivo = f'{APP}/{m.group(2)}.ts'
        if rota:
            mapa.setdefault(arquivo, set()).add(rota)
    return mapa


def grafo_reverso():
    """arquivo → quem o importa (só imports relativos dentro de src/app)."""
    reverso = {}
    for pasta, _, arquivos in os.walk(os.path.join(RAIZ, APP)):
        for nome in arquivos:
            if not nome.endswith('.ts'):
                continue
            caminho = os.path.join(pasta, nome)
            rel = os.path.relpath(caminho, RAIZ).replace(os.sep, '/')
            for imp in re.findall(r"""(?:from|import\()\s*['"](\.{1,2}/[^'"]+)['"]""", open(caminho, encoding='utf-8').read()):
                alvo = os.path.normpath(os.path.join(os.path.dirname(caminho), imp))
                alvo = os.path.relpath(alvo, RAIZ).replace(os.sep, '/')
                if not alvo.endswith('.ts'):
                    alvo += '.ts'
                reverso.setdefault(alvo, set()).add(rel)
    return reverso


def telas_afetadas(arquivo, reverso, componentes):
    """As rotas que dependem do arquivo; None se nenhuma (arquivo solto: na dúvida, tudo)."""
    vistos, fila, rotas = {arquivo}, [arquivo], set()
    while fila:
        atual = fila.pop()
        rotas |= componentes.get(atual, set())
        for quem in reverso.get(atual, ()):
            if quem not in vistos:
                vistos.add(quem)
                fila.append(quem)
    return rotas or None


def specs_navegador():
    pasta = os.path.join(RAIZ, FRONT, 'testes-navegador')
    return {n: open(os.path.join(pasta, n), encoding='utf-8').read() for n in sorted(os.listdir(pasta)) if n.endswith('.spec.ts')}


def jest_que_leem_codigo():
    saida = []
    for pasta, _, arquivos in os.walk(os.path.join(RAIZ, FRONT, 'testes')):
        for nome in arquivos:
            if nome.endswith('.spec.ts'):
                caminho = os.path.join(pasta, nome)
                if re.search(r'readFileSync|readdirSync', open(caminho, encoding='utf-8').read()):
                    saida.append(os.path.relpath(caminho, os.path.join(RAIZ, FRONT)).replace(os.sep, '/'))
    return sorted(saida)


def decidir(lista):
    """O que rodar, com o porquê (para o log e o resumo do PR)."""
    if not any(re.search(p, a) for a in lista for p in DO_FRONT):
        return {'frontend': False, 'tudo': False, 'motivo': ['nada do front-end mudou: os testes do front-end ficam de fora']}
    globais = [a for a in lista if any(re.search(p, a) for p in GLOBAIS)]
    if globais:
        return {'frontend': True, 'tudo': True, 'motivo': [f'arquivo global mudou: {a}' for a in globais[:5]]}

    componentes, reverso = rotas_por_componente(), grafo_reverso()
    rotas, specs_mudados, jest_mudados, fontes, motivo = set(), set(), set(), [], []
    for a in lista:
        if not a.startswith(f'{FRONT}/'):
            continue
        if a.startswith(f'{FRONT}/testes-navegador/') and a.endswith('.spec.ts'):
            specs_mudados.add(os.path.basename(a))
            motivo.append(f'teste no navegador mudou: {os.path.basename(a)}')
        elif a.startswith(f'{FRONT}/testes/') and a.endswith('.spec.ts'):
            jest_mudados.add(a[len(FRONT) + 1:])
            motivo.append(f'teste do Jest mudou: {a[len(FRONT) + 1:]}')
        elif a.startswith(f'{APP}/') and a.endswith('.ts'):
            r = telas_afetadas(a, reverso, componentes)
            if r is None:
                return {'frontend': True, 'tudo': True, 'motivo': [f'não deu para ligar a uma tela: {a} (na dúvida, tudo)']}
            rotas |= r
            fontes.append(a[len(FRONT) + 1:])
            motivo.append(f'{a[len(APP) + 1:]} → {", ".join("/" + x for x in sorted(r))}')
        elif a.startswith(f'{FRONT}/testes-navegador/fotos') or a.endswith('.md'):
            continue
        else:
            return {'frontend': True, 'tudo': True, 'motivo': [f'arquivo do front-end sem regra: {a} (na dúvida, tudo)']}

    specs = specs_navegador()
    arquivos_nav, titulos = set(specs_mudados), []
    for rota in sorted(rotas):
        abre = re.compile(r"""['"`]/""" + re.escape(rota) + r"""(?=['"`?#/])""")
        arquivos_nav |= {n for n, t in specs.items() if n != 'telas.spec.ts' and abre.search(t)}
        titulos += [f'/{rota} abre sem erro', f'/{rota}: sem rolagem']
    if rotas:
        arquivos_nav.add('celular.spec.ts')
    partes = [escapar(n) for n in sorted(arquivos_nav)] + [escapar(t) for t in titulos]
    grep = '|'.join(partes)
    jest = sorted(set(fontes) | jest_mudados | set(jest_que_leem_codigo()))
    return {'frontend': True, 'tudo': False, 'motivo': motivo, 'rotas': sorted(rotas), 'arquivos_nav': sorted(arquivos_nav),
            'grep': grep, 'jest': jest}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--base', help='ref da base (ex.: origin/develop); sem ela, tudo')
    p.add_argument('--arquivos', nargs='*', help='simula a lista de arquivos mudados (sem git)')
    p.add_argument('--local', action='store_true', help='soma o que ainda não foi commitado (#286)')
    p.add_argument('--executar', action='store_true', help='roda o Jest e o navegador só nos afetados (#286)')
    a = p.parse_args()
    if a.arquivos is not None:
        lista = a.arquivos
    elif a.base:
        lista = mudados(a.base, a.local)
    else:
        lista = None
    d = decidir(lista) if lista is not None else {'frontend': True, 'tudo': True, 'motivo': ['sem base (push): tudo']}

    playwright = '' if d['tudo'] else (f'--grep "{d["grep"]}"' if d.get('grep') else '')
    # O jest.config.js só olha testes/ (roots); para seguir os imports até src/, a seleção põe src/ também (não há
    # teste dentro de src/, então nada a mais roda). As aspas protegem o <rootDir> do shell.
    raizes = "--roots '<rootDir>/testes' --roots '<rootDir>/src'"
    jest = '' if d['tudo'] else (f"{raizes} --findRelatedTests {' '.join(d['jest'])} --passWithNoTests" if d.get('jest') else '--listTests')
    linhas = ['## Testes do front-end neste PR (#255)', '']
    if not d['frontend']:
        linhas.append('Nada do front-end mudou: Jest e navegador ficam de fora.')
    elif d['tudo']:
        linhas.append('**Tudo**, porque:')
    else:
        linhas.append(f'**Só os afetados.** Telas: {", ".join("/" + r for r in d.get("rotas", [])) or "nenhuma"}. '
                      f'Navegador: {", ".join(d.get("arquivos_nav", [])) or "nenhum arquivo"}. Jest: {len(d.get("jest", []))} arquivo(s). Por quê:')
    linhas += [f'- {m}' for m in d['motivo']]
    texto = '\n'.join(linhas)
    print(texto)
    print(f'\nplaywright test {playwright}\njest {jest}')

    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as f:
            f.write(f"frontend={'true' if d['frontend'] else 'false'}\n")
            f.write(f"tudo={'true' if d['tudo'] else 'false'}\n")
            f.write(f'playwright={playwright}\n')
            f.write(f'jest={jest}\n')
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as f:
            f.write(texto + '\n')
    return executar(d) if a.executar else 0


if __name__ == '__main__':
    sys.exit(main())
