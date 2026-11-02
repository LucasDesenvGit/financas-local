#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - Gerenciador no Linux (#138, #277; ADR 0033 e ADR 0052)
#
#  - Compila o gerenciador em Go (src/gerenciador-go, Wails) em publicado/gerenciador-go,
#    com o icone.png ao lado (o "Iniciar com o sistema" aponta para ele)
#  - Cria o atalho br.financaslocal.Gerenciador.desktop no menu de aplicativos (o GTK 4
#    tira dele o icone da janela) e na area de trabalho do usuario (padrao freedesktop)
#  - Sem o compilador de C, o GTK 4 ou o WebKitGTK 6.0 (o gerenciador em Go precisa de
#    cgo), diz o comando que instala o que falta e, ate a saida do .NET (#86), instala
#    o gerenciador em C# (Avalonia) em publicado/gerenciador, como antes
#
# Nao precisa de root: o servico do site e do systemd do proprio usuario
# (3-publicar-atualizar.sh), e o gerenciador liga, desliga e copia o banco
# sem senha. Fica fora de publicado/api, que a publicacao espelha.
# Rode de novo para atualizar o gerenciador; se ele estiver aberto, e fechado
# antes. No Windows, use o scripts/windows/instalar-gerenciador.ps1.
#
# Uso:
#   scripts/linux/instalar-gerenciador.sh
#   scripts/linux/instalar-gerenciador.sh --destino /outra/pasta
#   scripts/linux/instalar-gerenciador.sh --sem-atalho-na-area-de-trabalho
#   scripts/linux/instalar-gerenciador.sh --em-csharp    # o gerenciador em C#, mesmo com tudo para o em Go
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/../geral/comum.sh"

destino=""
atalho_area_de_trabalho=1
forcar_csharp=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --destino)
            [[ $# -ge 2 ]] || erro_sai '--destino precisa de uma pasta'
            destino="$2"; shift ;;
        --sem-atalho-na-area-de-trabalho) atalho_area_de_trabalho=0 ;;
        --em-csharp) forcar_csharp=1 ;;
        *) erro_sai "Argumento desconhecido: $1 (use --destino <pasta>, --sem-atalho-na-area-de-trabalho ou --em-csharp)" ;;
    esac
    shift
done

[[ "$SO" == linux ]] || erro_sai 'Este script e para o Linux. No Windows: powershell -ExecutionPolicy Bypass -File scripts\windows\instalar-gerenciador.ps1'

# Textos do atalho com acento, em bytes UTF-8 (os scripts ficam em ASCII).
nome="$(printf 'Finan\303\247as Local')"
comentario="$(printf 'Ver se o site est\303\241 no ar, ligar, desligar e copiar o banco')"
pasta_aplicativos="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
pasta_inicio="${XDG_CONFIG_HOME:-$HOME/.config}/autostart"
# Os nomes dos atalhos: o do em Go tem de ser o id do aplicativo (ApplicationID do Wails, sistema_linux.go); o antigo,
# do em C#, sai quando o em Go entra, para o menu nao mostrar dois. O do "Iniciar com o sistema" e o mesmo nos dois
# (NomeDoInicio, internal/preferencias/freedesktop.go).
atalho_go='br.financaslocal.Gerenciador.desktop'
atalho_csharp='financaslocal-gerenciador.desktop'
atalho_inicio="$pasta_inicio/financaslocal-gerenciador.desktop"

configurar_ambiente_build

if ! systemctl --user cat financaslocal.service >/dev/null 2>&1; then
    aviso 'O servico financaslocal.service nao esta registrado: o gerenciador vai dizer que o site nao esta instalado.'
    aviso 'Instale o site antes com scripts/geral/instalar.sh.'
fi

# ------------------------------------------------------------------ O que o gerenciador em Go precisa
# Compilar com cgo pede um compilador de C e os cabecalhos do GTK 4 e do WebKitGTK 6.0 (ADR 0052); quem tem os
# cabecalhos tem tambem as bibliotecas, que a janela usa ao abrir.
faltando_para_o_go() {
    local faltam=()
    command -v cc >/dev/null 2>&1 || command -v gcc >/dev/null 2>&1 || faltam+=('compilador de C')
    if ! command -v pkg-config >/dev/null 2>&1; then
        faltam+=('pkg-config')
    else
        pkg-config --exists gtk4 || faltam+=('GTK 4 (cabecalhos)')
        pkg-config --exists webkitgtk-6.0 || faltam+=('WebKitGTK 6.0 (cabecalhos)')
    fi
    command -v go >/dev/null 2>&1 || faltam+=('Go (rode antes o scripts/geral/instalar.sh)')
    local IFS=','
    printf '%s' "${faltam[*]}"
}

# O comando de instalar os pacotes, pela distribuicao (/etc/os-release: ID e ID_LIKE).
comando_dos_pacotes() {
    local id="" parecido=""
    if [[ -r /etc/os-release ]]; then
        id="$(sed -n 's/^ID=//p' /etc/os-release | tr -d '"')"
        parecido="$(sed -n 's/^ID_LIKE=//p' /etc/os-release | tr -d '"')"
    fi
    case " $id $parecido " in
        *" debian "*|*" ubuntu "*) echo 'sudo apt install gcc pkg-config libgtk-4-dev libwebkitgtk-6.0-dev' ;;
        *" fedora "*|*" rhel "*) echo 'sudo dnf install gcc pkgconf-pkg-config gtk4-devel webkitgtk6.0-devel' ;;
        *" arch "*) echo 'sudo pacman -S --needed gcc pkgconf gtk4 webkitgtk-6.0' ;;
        *) echo 'instale o gcc, o pkg-config e os pacotes de desenvolvimento do GTK 4 e do WebKitGTK 6.0 da sua distribuicao' ;;
    esac
}

em_go=1
if [[ "$forcar_csharp" -eq 1 ]]; then
    em_go=0
    aviso '--em-csharp: instalando o gerenciador em C#.'
else
    faltam="$(faltando_para_o_go)"
    if [[ -n "$faltam" ]]; then
        em_go=0
        aviso "Para o gerenciador novo (em Go) falta: ${faltam//,/, }."
        aviso "Para instalar: $(comando_dos_pacotes)"
        aviso 'e rode este script de novo. Por enquanto, vai o gerenciador em C#, que faz o mesmo.'
    fi
fi

if [[ "$em_go" -eq 1 ]]; then
    [[ -n "$destino" ]] || destino="$PastaPublicado/gerenciador-go"
    executavel_nome='financas-gerenciador'
else
    [[ -n "$destino" ]] || destino="$PastaPublicado/gerenciador"
    executavel_nome='Financas.Gerenciador.Linux'
fi
garantir_pasta "$destino"
destino="$(cd "$destino" && pwd)"
executavel="$destino/$executavel_nome"

# ------------------------------------------------------------------ Gerenciador aberto
# Fecha os dois (em Go e em C#) que rodam das pastas do Financas Local: o executavel e trocado pela publicacao, e o
# em C# sai do menu quando o em Go entra.
fechados=0
for exe in /proc/[0-9]*/exe; do
    alvo="$(readlink "$exe" 2>/dev/null || true)"
    case "$alvo" in
        "$executavel"|"$PastaPublicado/gerenciador/Financas.Gerenciador.Linux"|"$PastaPublicado/gerenciador-go/financas-gerenciador")
            pid="${exe#/proc/}"; pid="${pid%/exe}"
            kill "$pid" 2>/dev/null && fechados=$((fechados + 1)) ;;
    esac
done
if [[ "$fechados" -gt 0 ]]; then
    etapa 'Fechando o gerenciador aberto'
    sleep 1
    ok 'gerenciador fechado'
fi

# ------------------------------------------------------------------ Compilacao ou publicacao
if [[ "$em_go" -eq 1 ]]; then
    # Como o README do modulo: com cgo, sem F12 nem menu do botao direito (-tags production). A versao e a do
    # Directory.Build.props, como a do site.
    etapa "Compilando o gerenciador em Go em $destino"
    versao="$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' "$Raiz/Directory.Build.props" | head -n1)"
    (cd "$Raiz/src/gerenciador-go" && CGO_ENABLED=1 go build -tags production -trimpath \
        -ldflags "-s -w -X main.versao=$versao" -o "$executavel" .)
    cp "$Raiz/src/gerenciador-go/icone.png" "$destino/icone.png"
else
    # Self-contained: nao depende do .NET do sistema nem do de ferramentas/ (que o 1-preparar-ambiente.sh pode trocar
    # de versao).
    etapa "Publicando o gerenciador em C# em $destino"
    dotnet publish "$Raiz/src/gerenciador/Financas.Gerenciador.Linux/Financas.Gerenciador.Linux.csproj" -c Release \
        -r "linux-$ARCH" --self-contained true -o "$destino" --nologo
    find "$destino" -maxdepth 1 -name '*.pdb' -delete
fi
[[ -x "$executavel" ]] || erro_sai "Executavel nao encontrado em $executavel"
ok 'gerenciador pronto'

# ------------------------------------------------------------------ Atalhos
# Exec= entre aspas, com \ " ` $ escapados (especificacao Desktop Entry).
aspas_desktop() {
    local valor="$1"
    valor="${valor//\\/\\\\}"; valor="${valor//\"/\\\"}"; valor="${valor//\`/\\\`}"; valor="${valor//\$/\\\$}"
    printf '"%s"' "$valor"
}

escrever_atalho() {
    local classe='Financas.Gerenciador.Linux'
    [[ "$em_go" -eq 1 ]] && classe='br.financaslocal.Gerenciador'
    cat > "$1" <<EOF
[Desktop Entry]
Type=Application
Name=Gerenciador do $nome
GenericName=Gerenciador do site
Comment=$comentario
Exec=$(aspas_desktop "$executavel")
Icon=$destino/icone.png
Terminal=false
Categories=Office;Finance;
StartupWMClass=$classe
EOF
    chmod 755 "$1"
}

etapa 'Criando o atalho no menu de aplicativos'
garantir_pasta "$pasta_aplicativos"
if [[ "$em_go" -eq 1 ]]; then
    atalho="$pasta_aplicativos/$atalho_go"
    rm -f "$pasta_aplicativos/$atalho_csharp"
else
    atalho="$pasta_aplicativos/$atalho_csharp"
    rm -f "$pasta_aplicativos/$atalho_go"
fi
escrever_atalho "$atalho"
if command -v desktop-file-validate >/dev/null 2>&1; then
    desktop-file-validate "$atalho" || erro_sai "Atalho invalido: $atalho"
fi
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$pasta_aplicativos" 2>/dev/null || true
ok "menu de aplicativos: Gerenciador do $nome"

if [[ "$atalho_area_de_trabalho" -eq 1 ]]; then
    etapa 'Criando o atalho na area de trabalho'
    area="$(xdg-user-dir DESKTOP 2>/dev/null || true)"
    [[ -n "$area" ]] || area="$HOME/Desktop"
    if [[ -d "$area" && "$area" != "$HOME" ]]; then
        # O mesmo nome de antes: troca o atalho do em C#, em vez de deixar dois.
        atalho_area="$area/$atalho_csharp"
        escrever_atalho "$atalho_area"
        # GNOME/Nautilus: sem isto, o atalho abre como "nao confiavel" ate a pessoa permitir.
        command -v gio >/dev/null 2>&1 && gio set "$atalho_area" metadata::trusted true 2>/dev/null || true
        ok "area de trabalho: $atalho_area"
    else
        aviso 'Area de trabalho nao encontrada: so o atalho do menu foi criado.'
    fi
fi

# "Iniciar com o sistema" ligado: o atalho do inicio passa a abrir este gerenciador, com o texto que ele mesmo grava
# (AtalhoDoInicio, internal/preferencias/freedesktop.go, e o Preferencias.cs do Linux).
if [[ -f "$atalho_inicio" ]]; then
    cat > "$atalho_inicio" <<EOF
[Desktop Entry]
Type=Application
Name=Gerenciador do $nome
Comment=$(printf 'Avisa se o site do Finan\303\247as Local cair')
Exec=$(aspas_desktop "$executavel") --bandeja
Icon=$destino/icone.png
Terminal=false
X-GNOME-Autostart-enabled=true
EOF
    ok "iniciar com o sistema: passa a abrir $executavel"
fi

echo
ok "Pronto: abra pelo menu de aplicativos (Gerenciador do $nome) ou por $executavel"
