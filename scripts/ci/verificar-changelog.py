#!/usr/bin/env python3
"""
#294: o CHANGELOG.md tem uma seção por versão e, dentro dela, um título de cada tipo.
- Cada PR sobe a versão (ADR 0039), e a entrada dele vai na seção da versão dele: "## [X.Y.Z] — AAAA-MM-DD ·
  desenvolvimento" (criada no topo, se ainda não existe), DENTRO do "### Funcionalidades", "### Correções"… que já
  existe nela. Publicar a versão é tirar o "· desenvolvimento" do título.
- Esta conferência falha com: título de tipo repetido na mesma versão; versão repetida; versões fora de ordem (a mais
  nova em cima); versão acima da do Directory.Build.props.
- Antes, cada PR abria a própria "### Correções" no topo de um "Não publicado" que juntava tudo, e a seção da 2.17.0
  publicada nem voltou para o develop; as notas e a janela "O que mudou" do gerenciador ficavam repetidas e longas.

Uso na CI:   python3 scripts/ci/verificar-changelog.py             (falha com título repetido na mesma versão)
Corrigir:    python3 scripts/ci/verificar-changelog.py --corrigir  (junta as repetidas, na ordem em que cada uma
                                                                      apareceu primeiro, e mantém as entradas)
"""
import io
import os
import re
import sys

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
ARQUIVO = os.path.join(RAIZ, 'CHANGELOG.md')


def partes(texto):
    """[(cabeçalho da versão ou None, [linhas antes do primeiro ###], [(título ###, [linhas])])]."""
    versoes = []
    atual = None
    for linha in texto.split('\n'):
        if linha.startswith('## '):
            atual = [linha, [], []]
            versoes.append(atual)
        elif atual is None:
            versoes.append(atual := [None, [linha], []])
        elif linha.startswith('### '):
            atual[2].append([linha.strip(), []])
        elif atual[2]:
            atual[2][-1][1].append(linha)
        else:
            atual[1].append(linha)
    return versoes


def repetidas(versoes):
    achadas = []
    for cabecalho, _, secoes in versoes:
        vistos = set()
        for titulo, _ in secoes:
            if titulo in vistos:
                achadas.append((cabecalho, titulo))
            vistos.add(titulo)
    return achadas


def juntar(versoes):
    linhas = []
    for cabecalho, antes, secoes in versoes:
        if cabecalho is not None:
            linhas.append(cabecalho)
        linhas.extend(antes)
        ordem, corpo = [], {}
        for titulo, conteudo in secoes:
            if titulo not in corpo:
                ordem.append(titulo)
                corpo[titulo] = []
            # Sem as linhas em branco das pontas: entre as partes juntadas não fica buraco.
            while conteudo and not conteudo[-1].strip():
                conteudo = conteudo[:-1]
            while conteudo and not conteudo[0].strip():
                conteudo = conteudo[1:]
            corpo[titulo].extend(conteudo)
        for titulo in ordem:
            linhas.append(titulo)
            linhas.extend(corpo[titulo])
            linhas.append('')
    return '\n'.join(linhas).rstrip('\n') + '\n'


def problemas_de_versao(versoes):
    """Versão repetida, fora de ordem (a mais nova em cima) ou acima da do Directory.Build.props."""
    props = io.open(os.path.join(RAIZ, 'Directory.Build.props'), encoding='utf-8').read()
    atual = re.search(r'<Version>(\d+)\.(\d+)\.(\d+)</Version>', props)
    atual = tuple(int(x) for x in atual.groups()) if atual else None
    numeros = []
    for cabecalho, _, _ in versoes:
        m = re.match(r'## \[(\d+)\.(\d+)\.(\d+)\]', cabecalho or '')
        if m:
            numeros.append((tuple(int(x) for x in m.groups()), cabecalho))
    achados, vistos = [], set()
    for (numero, cabecalho), (seguinte, _) in zip(numeros, numeros[1:] + [((-1,), None)]):
        if numero in vistos:
            achados.append(f'versão repetida: {cabecalho}')
        vistos.add(numero)
        if seguinte >= numero:
            achados.append(f'fora de ordem (a mais nova vai em cima): {cabecalho}')
        if atual and numero > atual:
            achados.append(f'acima da versão do Directory.Build.props ({".".join(map(str, atual))}): {cabecalho}')
    return achados


def main():
    texto = io.open(ARQUIVO, encoding='utf-8').read().replace('\r\n', '\n')
    versoes = partes(texto)
    achadas = repetidas(versoes)
    erros_versao = problemas_de_versao(versoes)
    if erros_versao and '--corrigir' not in sys.argv:
        print('CHANGELOG.md com seções de versão erradas:')
        for e in erros_versao:
            print(f'  - {e}')
        return 1
    if '--corrigir' in sys.argv:
        if achadas:
            io.open(ARQUIVO, 'w', encoding='utf-8', newline='\n').write(juntar(versoes))
            print(f'CHANGELOG.md: {len(achadas)} seção(ões) repetida(s) juntada(s).')
        else:
            print('CHANGELOG.md: nada a juntar.')
        return 0
    if achadas:
        print('CHANGELOG.md tem título repetido na mesma versão (a entrada nova vai DENTRO da seção que já existe):')
        for cabecalho, titulo in achadas:
            print(f'  - {cabecalho}: "{titulo}"')
        print('Para juntar: python3 scripts/ci/verificar-changelog.py --corrigir')
        return 1
    print('OK: CHANGELOG.md sem título repetido na mesma versão.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
