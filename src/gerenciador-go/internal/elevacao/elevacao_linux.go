package elevacao

// Executar: no Linux, nada pede administrador (o serviço é do systemd do próprio usuário), e o gerenciador roda as
// ações direto (sistema_linux.go). Não é chamado.
func Executar(...string) Resultado { return Falhou }
