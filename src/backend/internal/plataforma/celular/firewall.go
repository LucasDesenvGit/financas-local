package celular

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Firewall abre e fecha a porta do site para a rede de casa. Declarado aqui, por quem usa; o do sistema vem do
// FirewallDoSistema (Windows: netsh; Linux: só avisa, porque o serviço roda na conta da pessoa, sem root).
type Firewall interface {
	// Abrir deixa a rede de casa chegar à porta no IP dado, e só ela. Chamado de novo, troca a regra (não duplica).
	Abrir(ctx context.Context, e Endereco, porta int) error
	// Fechar apaga a regra. Sem regra, não faz nada.
	Fechar(ctx context.Context) error
	// Avisos diz o que a pessoa ainda precisa fazer para o celular chegar ao site (o perfil de rede do Windows, o
	// firewall do Linux). Vazio: nada a fazer.
	Avisos(ctx context.Context, e Endereco, porta int) []string
}

// rodarComando roda um programa e devolve a saída junta (a saída e os erros); o teste troca por um falso.
type rodarComando func(ctx context.Context, programa string, argumentos ...string) ([]byte, error)

func rodarDeVerdade(ctx context.Context, programa string, argumentos ...string) ([]byte, error) {
	return exec.CommandContext(ctx, programa, argumentos...).CombinedOutput()
}

// nomeDaRegra é o nome da regra no firewall do Windows. Sem espaço nem acento: o netsh lê a própria linha de comando, e
// "name=Finanças Local" entre aspas é onde ele costuma se perder.
const nomeDaRegra = "FinancasLocal-celular"

// netsh é o firewall do Windows pelo netsh advfirewall. O serviço roda como LocalSystem, que pode mexer no firewall; em
// desenvolvimento, sem administrador, o Abrir falha e diz isso.
type netsh struct{ rodar rodarComando }

// Abrir apaga a regra antiga (se houver) e cria outra: entrada, TCP, só na porta e no IP do site, só no perfil
// Particular e só para a sub-rede local (remoteip=localsubnet). Numa rede marcada como Pública (Wi-Fi de café,
// aeroporto) a porta fica fechada.
func (n netsh) Abrir(ctx context.Context, e Endereco, porta int) error {
	if err := n.Fechar(ctx); err != nil {
		return err
	}
	saida, err := n.rodar(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
		"name="+nomeDaRegra, "dir=in", "action=allow", "protocol=TCP", "enable=yes",
		"localip="+e.IP, "localport="+strconv.Itoa(porta), "remoteip=localsubnet", "profile=private")
	if err != nil {
		return fmt.Errorf("criar a regra do firewall do Windows (o site precisa rodar como serviço, o normal da instalação): %w: %s", err, limparSaida(saida))
	}
	return nil
}

// Fechar só apaga a regra que existe: o "show rule" sai com erro quando não há regra com o nome (o texto da saída muda
// com o idioma do Windows; o código de saída, não).
func (n netsh) Fechar(ctx context.Context) error {
	if _, err := n.rodar(ctx, "netsh", "advfirewall", "firewall", "show", "rule", "name="+nomeDaRegra); err != nil {
		return nil
	}
	saida, err := n.rodar(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name="+nomeDaRegra)
	if err != nil {
		return fmt.Errorf("apagar a regra do firewall do Windows: %w: %s", err, limparSaida(saida))
	}
	return nil
}

// AvisoRedePublica é o aviso de quando o Windows marcou a rede do IP como Pública: a regra só vale no perfil Particular.
const AvisoRedePublica = "O Windows marcou esta rede como Pública, e nela a porta continua fechada. Se é a rede da sua casa, mude para Particular: Configurações › Rede e Internet › (Wi-Fi ou Ethernet) › Perfil de rede › Particular."

// Avisos olha o perfil da rede do IP (Get-NetConnectionProfile). O IP já foi conferido (só dígitos e pontos), então
// pode ir no comando. Sem resposta (PowerShell que falha), não avisa nada: o certo é não inventar problema.
func (n netsh) Avisos(ctx context.Context, e Endereco, _ int) []string {
	comando := "(Get-NetConnectionProfile -InterfaceIndex (Get-NetIPAddress -IPAddress '" + e.IP + "' -ErrorAction Stop).InterfaceIndex -ErrorAction Stop).NetworkCategory"
	saida, err := n.rodar(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", comando)
	if err != nil {
		return []string{}
	}
	if strings.EqualFold(strings.TrimSpace(string(saida)), "Public") {
		return []string{AvisoRedePublica}
	}
	return []string{}
}

// limparSaida corta a saída do comando para caber na mensagem de erro.
func limparSaida(saida []byte) string {
	texto := strings.Join(strings.Fields(string(saida)), " ")
	if len(texto) > 300 {
		texto = texto[:300] + "…"
	}
	return texto
}

// linux é o firewall do Linux: o serviço roda na conta da pessoa (systemctl --user), sem root, então não mexe no ufw nem
// no firewalld. Só descobre se um deles está ligado e diz o comando, com sudo, para quem decide rodar.
type linux struct {
	rodar rodarComando
	// lerArquivo lê o /etc/ufw/ufw.conf (que qualquer conta lê); o teste troca.
	lerArquivo func(caminho string) ([]byte, error)
}

func (linux) Abrir(context.Context, Endereco, int) error { return nil }
func (linux) Fechar(context.Context) error               { return nil }

// Avisos diz o comando quando o ufw ou o firewalld estão ligados. Sem firewall ligado, a porta já está aberta para a
// rede (o padrão da maioria das distribuições de computador), e não há o que avisar.
func (l linux) Avisos(ctx context.Context, e Endereco, porta int) []string {
	avisos := []string{}
	if conf, err := l.lerArquivo("/etc/ufw/ufw.conf"); err == nil && ufwLigado(string(conf)) {
		avisos = append(avisos, fmt.Sprintf("O firewall ufw está ligado. Para o celular chegar ao site, rode uma vez no terminal: sudo ufw allow from %s to %s port %d proto tcp", e.Rede, e.IP, porta))
	}
	if saida, err := l.rodar(ctx, "systemctl", "is-active", "firewalld"); err == nil && strings.TrimSpace(string(saida)) == "active" {
		avisos = append(avisos, fmt.Sprintf("O firewalld está ligado. Para o celular chegar ao site, rode uma vez no terminal: sudo firewall-cmd --permanent --add-rich-rule='rule family=\"ipv4\" source address=\"%s\" port port=\"%d\" protocol=\"tcp\" accept' && sudo firewall-cmd --reload", e.Rede, porta))
	}
	return avisos
}

// ufwLigado lê o ENABLED=yes do ufw.conf. Comentário com # não conta, e vale a última linha, porque o arquivo é lido
// pelo shell (". /etc/ufw/ufw.conf") nos scripts do ufw.
func ufwLigado(conf string) bool {
	ligado := false
	for _, linha := range strings.Split(conf, "\n") {
		linha = strings.TrimSpace(linha)
		if strings.HasPrefix(linha, "#") {
			continue
		}
		if chave, valor, ok := strings.Cut(linha, "="); ok && strings.TrimSpace(chave) == "ENABLED" {
			ligado = strings.EqualFold(strings.Trim(strings.TrimSpace(valor), `"'`), "yes")
		}
	}
	return ligado
}
