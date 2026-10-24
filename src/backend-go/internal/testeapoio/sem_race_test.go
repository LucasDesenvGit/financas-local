//go:build !race

package testeapoio

// comRace diz se o teste roda com -race, que deixa o SQLite (Go puro) dezenas de vezes mais lento.
const comRace = false
