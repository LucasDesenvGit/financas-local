#!/usr/bin/env python3
"""
#286 (ADR 0047): `npm run test:afetados`, antes do push. Roda o Jest inteiro (~8 s), o build e, no navegador, só os
testes das telas afetadas pelo que mudou desde a base, inclusive o que ainda não foi commitado. A CI roda tudo (#774).

- Um arquivo de src/app afeta as telas (rotas de app.routes.ts) que o importam, direta ou indiretamente (grafo de
  imports). Rodam os arquivos de teste que abrem essas telas (goto('/rota') / abrir(page, '/rota')), as linhas de
  telas.spec.ts dessas rotas, o celular.spec.ts e os arquivos de teste que mudaram.
- Template e CSS em arquivo próprio (templateUrl/styleUrl, #475, ADR 0059) contam como o .ts do componente que os
  aponta.
- Navegador inteiro: arquivo global (estilos, core/, layout/, rotas, index.html, configuração, apoio dos testes no
  navegador, a CSP) ou arquivo do front-end que não dá para ligar a uma tela. Na dúvida, tudo.
- Nada do front-end mudou: não roda nada.

Rodar:    cd src/frontend && NAVEGADOR=chromium npm run test:afetados   (com o Chromium da CI)
Ver:      python3 scripts/dev/testes-afetados.py                        (só mostra o que rodaria; --base origin/main)
Simular:  python3 scripts/dev/testes-afetados.py --arquivos a.ts b.css  (sem git)
"""
import argparse
import os
import re
import shutil
import subprocess
import sys

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
FRONT = 'src/frontend'
APP = f'{FRONT}/src/app'

# Mudou um destes: o navegador roda inteiro (afeta todas as telas ou o próprio jeito de testar).
GLOBAIS = [
    r'^src/frontend/src/styles\.css$', r'^src/frontend/src/index\.html$', r'^src/frontend/src/main\.ts$',
    r'^src/frontend/src/app/app\.[^/]+\.ts$', r'^src/frontend/src/app/app\.component\.ts$',
    r'^src/frontend/src/app/core/', r'^src/frontend/src/app/layout/',
    r'^src/frontend/(package(-lock)?\.json|angular\.json|tsconfig[^/]*\.json|playwright\.config\.ts)$',
    r'^src/frontend/testes/navegador/(base\.ts|api-falsa\.ts|servidor\.mjs)$',
    r'^src/frontend/public/', r'^src/frontend/src/assets/',
    # A CSP que os testes no navegador aplicam vem daqui (ADR 0041; do Go desde a #86).
    r'^src/backend/internal/plataforma/servidor/intermediarios\.go$',
]
# O que conta como "front-end" (fora disso, não roda nada).
DO_FRONT = [r'^src/frontend/', r'^src/backend/internal/plataforma/servidor/intermediarios\.go$']


def escapar(texto):
    """Escapa só o que é especial na regex do JavaScript (o --grep do Playwright)."""
    especiais = set('.*+?^${}()|[]\\/')
    return ''.join('\\' + c if c in especiais else c for c in texto)


def git(*args):
    return subprocess.run(['git', *args], cwd=RAIZ, capture_output=True, text=True, check=True).stdout


def mudados(base):
    """Arquivos mudados desde a base (o que o merge leva), mais o que ainda não foi commitado (alterado e novo)."""
    lista = git('diff', '--name-only', f'{base}...HEAD').splitlines()
    lista += git('diff', '--name-only', 'HEAD').splitlines()
    lista += git('ls-files', '--others', '--exclude-standard').splitlines()
    return sorted({l.strip() for l in lista if l.strip()})


def executar(d):
    """Roda o Jest inteiro, o build e o navegador (inteiro ou só nos afetados). Devolve o código de saída."""
    if not d['frontend']:
        print('\nNada do front-end mudou: nenhum teste a rodar.')
        return 0
    front = os.path.join(RAIZ, FRONT)
    # Node direto com o executável de cada ferramenta, sem npx/npm: no Windows eles são .cmd e passam pelo cmd.exe,
    # que lê o "|" do --grep como pipe.
    node = shutil.which('node') or 'node'

    def ferramenta(*caminho):
        return [node, os.path.join(front, 'node_modules', *caminho)]

    passos = [('Jest (tudo)', ferramenta('jest', 'bin', 'jest.js'))]
    if d['tudo'] or d['grep']:
        filtro = [] if d['tudo'] else ['--grep', d['grep']]
        passos += [('build de produção', [*ferramenta('@angular', 'cli', 'bin', 'ng.js'), 'build', '--configuration', 'production']),
                   (f'navegador ({"tudo" if d["tudo"] else "afetados"})', [*ferramenta('@playwright', 'test', 'cli.js'), 'test', *filtro])]
    for nome, comando in passos:
        print(f'\n==> {nome}', flush=True)
        if subprocess.run(comando, cwd=front).returncode != 0:
            print(f'\nFalhou: {nome}.')
            return 1
    print('\nTudo certo. O navegador inteiro roda na CI, no PR.')
    return 0


def rotas_por_componente():
    """'src/frontend/src/app/paginas/<grupo>/x.component.ts' → ['rota', …] (a rota "manual/:capitulo" conta como "manual")."""
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
    pasta = os.path.join(RAIZ, FRONT, 'testes', 'navegador')
    return {n: open(os.path.join(pasta, n), encoding='utf-8').read() for n in sorted(os.listdir(pasta)) if n.endswith('.spec.ts')}


def dono(arquivo):
    """#475: o .ts do componente que aponta o .html/.css em templateUrl/styleUrl(s), na mesma pasta; None se nenhum."""
    pasta, nome = os.path.split(arquivo)
    absoluta = os.path.join(RAIZ, pasta)
    if not os.path.isdir(absoluta):
        return None
    aponta = re.compile(r"""\b(?:templateUrl|styleUrls?)\s*:\s*\[?[^\]]*?['"]\./""" + re.escape(nome) + r"""['"]""")
    for ts in sorted(os.listdir(absoluta)):
        if ts.endswith('.ts') and aponta.search(open(os.path.join(absoluta, ts), encoding='utf-8').read()):
            return f'{pasta}/{ts}'
    return None


def decidir(lista):
    """O que rodar no navegador, com o porquê."""
    if not any(re.search(p, a) for a in lista for p in DO_FRONT):
        return {'frontend': False, 'tudo': False, 'motivo': ['nada do front-end mudou']}
    globais = [a for a in lista if any(re.search(p, a) for p in GLOBAIS)]
    if globais:
        return {'frontend': True, 'tudo': True, 'motivo': [f'arquivo global mudou: {a}' for a in globais[:5]]}

    componentes, reverso = rotas_por_componente(), grafo_reverso()
    rotas, specs_mudados, motivo = set(), set(), []
    for a in lista:
        if not a.startswith(f'{FRONT}/'):
            continue
        if a.startswith(f'{APP}/') and a.endswith(('.html', '.css')):
            ts = dono(a)
            if ts is None:
                return {'frontend': True, 'tudo': True, 'motivo': [f'nenhum componente aponta para {a} (na dúvida, tudo)']}
            motivo.append(f'{a[len(APP) + 1:]} é o template ou o CSS de {ts[len(APP) + 1:]}')
            a = ts
        if a.startswith(f'{FRONT}/testes/navegador/') and a.endswith('.spec.ts'):
            specs_mudados.add(os.path.basename(a))
            motivo.append(f'teste no navegador mudou: {os.path.basename(a)}')
        elif a.startswith(f'{APP}/') and a.endswith('.ts'):
            r = telas_afetadas(a, reverso, componentes)
            if r is None:
                return {'frontend': True, 'tudo': True, 'motivo': [f'não deu para ligar a uma tela: {a} (na dúvida, tudo)']}
            rotas |= r
            motivo.append(f'{a[len(APP) + 1:]} → {", ".join("/" + x for x in sorted(r))}')
        elif a.startswith((f'{FRONT}/testes/jest/', f'{FRONT}/testes/navegador/fotos')) or a.endswith('.md'):
            continue  # o Jest roda inteiro; fotos e texto não mudam tela
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
    grep = '|'.join([escapar(n) for n in sorted(arquivos_nav)] + [escapar(t) for t in titulos])
    return {'frontend': True, 'tudo': False, 'motivo': motivo, 'rotas': sorted(rotas), 'arquivos_nav': sorted(arquivos_nav),
            'grep': grep}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--base', default='origin/develop', help='ref da base (padrão: origin/develop)')
    p.add_argument('--arquivos', nargs='*', help='simula a lista de arquivos mudados (sem git)')
    p.add_argument('--executar', action='store_true', help='roda o Jest inteiro, o build e o navegador nos afetados')
    a = p.parse_args()
    d = decidir(a.arquivos if a.arquivos is not None else mudados(a.base))

    if not d['frontend']:
        print('Nada do front-end mudou.')
    elif d['tudo']:
        print('Navegador: tudo, porque:')
    elif not d['grep']:
        print('Navegador: nada a rodar (só o Jest).')
    else:
        print(f'Navegador: telas {", ".join("/" + r for r in d["rotas"]) or "nenhuma"}; '
              f'arquivos {", ".join(d["arquivos_nav"]) or "nenhum"}. Por quê:')
    print('\n'.join(f'- {m}' for m in d['motivo']))
    if d.get('grep'):
        print(f'\nnpx playwright test --grep "{d["grep"]}"')
    return executar(d) if a.executar else 0


if __name__ == '__main__':
    sys.exit(main())
