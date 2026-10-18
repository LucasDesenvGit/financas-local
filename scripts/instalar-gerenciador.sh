#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - Gerenciador no Linux (#138, ADR 0033)
#
#  - Publica o gerenciador (Avalonia) em publicado/gerenciador, self-contained
#  - Cria o atalho no menu de aplicativos e na area de trabalho do usuario
#    (padrao freedesktop: vale no KDE, GNOME, XFCE...)
#
# Nao precisa de root: o servico do site e do systemd do proprio usuario
# (3-publicar-atualizar.sh), e o gerenciador liga, desliga e copia o banco
# sem senha. Fica fora de publicado/api, que a publicacao espelha.
# Rode de novo para atualizar o gerenciador; se ele estiver aberto, e fechado
# antes. No Windows, use o scripts/instalar-gerenciador.ps1.
#
# Uso:
#   scripts/instalar-gerenciador.sh
#   scripts/instalar-gerenciador.sh --destino /outra/pasta
#   scripts/instalar-gerenciador.sh --sem-atalho-na-area-de-trabalho
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"

destino="$PastaPublicado/gerenciador"
atalho_area_de_trabalho=1
while [[ $# -gt 0 ]]; do
    case "$1" in
        --destino)
            [[ $# -ge 2 ]] || erro_sai '--destino precisa de uma pasta'
            destino="$2"; shift ;;
        --sem-atalho-na-area-de-trabalho) atalho_area_de_trabalho=0 ;;
        *) erro_sai "Argumento desconhecido: $1 (use --destino <pasta> ou --sem-atalho-na-area-de-trabalho)" ;;
    esac
    shift
done

[[ "$SO" == linux ]] || erro_sai 'Este script e para o Linux. No Windows: powershell -ExecutionPolicy Bypass -File scripts\instalar-gerenciador.ps1'

garantir_pasta "$destino"
destino="$(cd "$destino" && pwd)"
projeto="$Raiz/src/gerenciador/Financas.Gerenciador.Linux/Financas.Gerenciador.Linux.csproj"
executavel="$destino/Financas.Gerenciador.Linux"
# Textos do atalho com acento, em bytes UTF-8 (os scripts ficam em ASCII).
nome="$(printf 'Finan\303\247as Local')"
comentario="$(printf 'Ver se o site est\303\241 no ar, ligar, desligar e copiar o banco')"

configurar_ambiente_build

if ! systemctl --user cat financaslocal.service >/dev/null 2>&1; then
    aviso 'O servico financaslocal.service nao esta registrado: o gerenciador vai dizer que o site nao esta instalado.'
    aviso 'Instale o site antes com scripts/instalar.sh.'
fi

# ------------------------------------------------------------------ Gerenciador aberto
# Fecha so o que roda desta pasta (o executavel e trocado pela publicacao).
fechados=0
for exe in /proc/[0-9]*/exe; do
    alvo="$(readlink "$exe" 2>/dev/null || true)"
    if [[ "$alvo" == "$executavel" ]]; then
        pid="${exe#/proc/}"; pid="${pid%/exe}"
        kill "$pid" 2>/dev/null && fechados=$((fechados + 1))
    fi
done
if [[ "$fechados" -gt 0 ]]; then
    etapa 'Fechando o gerenciador aberto'
    sleep 1
    ok 'gerenciador fechado'
fi

# ------------------------------------------------------------------ Publicacao
# Self-contained: nao depende do .NET do sistema nem do de ferramentas/ (que
# o 1-preparar-ambiente.sh pode trocar de versao).
etapa "Publicando o gerenciador em $destino"
dotnet publish "$projeto" -c Release -r "linux-$ARCH" --self-contained true -o "$destino" --nologo
find "$destino" -maxdepth 1 -name '*.pdb' -delete
[[ -x "$executavel" ]] || erro_sai "Executavel nao encontrado em $executavel"
ok 'gerenciador publicado'

# ------------------------------------------------------------------ Atalhos
# Exec= entre aspas, com \ " ` $ escapados (especificacao Desktop Entry).
aspas_desktop() {
    local valor="$1"
    valor="${valor//\\/\\\\}"; valor="${valor//\"/\\\"}"; valor="${valor//\`/\\\`}"; valor="${valor//\$/\\\$}"
    printf '"%s"' "$valor"
}

escrever_atalho() {
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
StartupWMClass=Financas.Gerenciador.Linux
EOF
    chmod 755 "$1"
}

etapa 'Criando o atalho no menu de aplicativos'
pasta_aplicativos="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
garantir_pasta "$pasta_aplicativos"
atalho="$pasta_aplicativos/financaslocal-gerenciador.desktop"
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
        atalho_area="$area/financaslocal-gerenciador.desktop"
        escrever_atalho "$atalho_area"
        # GNOME/Nautilus: sem isto, o atalho abre como "nao confiavel" ate a pessoa permitir.
        command -v gio >/dev/null 2>&1 && gio set "$atalho_area" metadata::trusted true 2>/dev/null || true
        ok "area de trabalho: $atalho_area"
    else
        aviso 'Area de trabalho nao encontrada: so o atalho do menu foi criado.'
    fi
fi

echo
ok "Pronto: abra pelo menu de aplicativos (Gerenciador do $nome) ou por $executavel"
