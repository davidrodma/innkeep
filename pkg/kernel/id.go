package kernel

import (
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // G505: exigido pelo UUID v5 (RFC 9562); ver DeterministicID
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// NewID gera um UUID versão 7 (RFC 9562).
//
// Por que v7 e não o v4 (aleatório) de sempre? O v7 começa com o timestamp em
// milissegundos, então IDs novos são sempre "maiores" que os antigos. Em um
// índice B-tree do PostgreSQL isso significa inserir sempre no fim da árvore
// (como um autoincremento), em vez de espalhar escritas pelo índice inteiro.
// Ao mesmo tempo o ID pode ser gerado em qualquer serviço, sem ir ao banco —
// requisito para eventos, que precisam de identidade antes de serem gravados.
//
// Dentro do MESMO milissegundo, a ordem é garantida por um contador de 12 bits
// no campo "rand_a" (Método 1 da seção 6.2 da RFC 9562): cada ID novo do mesmo
// milissegundo incrementa o contador. Se o contador estourar, avançamos o
// relógio lógico em 1 ms. A garantia vale dentro de um processo; entre
// processos diferentes, só a ordem por milissegundo.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand só falha se o sistema operacional estiver quebrado.
		panic(fmt.Sprintf("kernel: falha ao gerar bytes aleatórios: %v", err))
	}

	idClock.Lock()
	ms := uint64(time.Now().UnixMilli())
	if ms > idClock.lastMs {
		idClock.lastMs = ms
		// Começa em um ponto aleatório da metade inferior, deixando espaço
		// para incrementos no mesmo milissegundo.
		idClock.counter = uint16(b[6])<<4 | uint16(b[7]>>4)&0x0f
		idClock.counter &= 0x07ff
	} else {
		idClock.counter++
		if idClock.counter > 0x0fff {
			idClock.lastMs++
			idClock.counter = 0
		}
	}
	ms, counter := idClock.lastMs, idClock.counter
	idClock.Unlock()

	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = 0x70 | byte(counter>>8)&0x0f // versão 7 + 4 bits altos do contador
	b[7] = byte(counter)                // 8 bits baixos do contador
	b[8] = (b[8] & 0x3f) | 0x80         // variante RFC 9562
	return formatUUID(b)
}

// idClock guarda o último milissegundo e o contador usados por NewID.
var idClock struct {
	sync.Mutex
	lastMs  uint64
	counter uint16
}

// DeterministicID gera um UUID versão 5 (hash SHA-1 de namespace + nome).
//
// O mesmo par (namespace, name) produz SEMPRE o mesmo ID. É assim que uma
// reserva vinda do Booking.com ganha o mesmo ID interno toda vez que o canal
// reenvia a mensagem: a idempotência nasce da própria identidade.
//
// O SHA-1 aqui não protege nada: é o algoritmo que a RFC 9562 define para o
// UUID v5, e um ID não é segredo. Por isso o alerta do gosec é dispensado.
func DeterministicID(namespace, name string) string {
	h := sha1.New() //nolint:gosec // G401: ver o comentário acima
	h.Write([]byte("innkeep:" + namespace + ":"))
	h.Write([]byte(name))
	var b [16]byte
	copy(b[:], h.Sum(nil))
	b[6] = (b[6] & 0x0f) | 0x50 // versão 5
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b)
}

// IsID informa se value tem o formato canônico de UUID (8-4-4-4-12 hex).
func IsID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

func formatUUID(b [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}
