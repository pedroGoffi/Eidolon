// Package idgen gera correlation IDs únicos para rastrear cada request
// ponta a ponta (Nginx -> Core -> serviço interno -> Core -> client).
//
// Implementado apenas com crypto/rand da standard library para evitar
// dependência de módulos externos (google/uuid etc). Gera UUID v4
// (aleatório) — se quiser UUID v7 (ordenável por tempo, melhor para
// índices no ClickHouse), troque por github.com/google/uuid >= 1.6
// quando tiver acesso ao proxy de módulos do Go.
package idgen

import (
	"crypto/rand"
	"fmt"
)

// New gera um novo correlation ID no formato UUID v4.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremamente improvável (falha de entropia do SO). Fail closed
		// não se aplica aqui, mas não devemos silenciar o erro.
		panic("idgen: falha ao ler crypto/rand: " + err.Error())
	}

	// Ajusta os bits de versão (4) e variante (RFC 4122).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
