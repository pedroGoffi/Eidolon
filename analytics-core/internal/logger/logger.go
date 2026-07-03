// Package logger fornece logging estruturado (JSON, via log/slog) para
// operação do processo, e um Sink assíncrono para os RequestLogs que em
// produção vão para o ClickHouse.
//
// O Sink é uma interface de propósito: aqui implementamos um StdoutSink
// (sem dependências externas) que você troca por um ClickHouseSink real
// (usando clickhouse-go) quando tiver acesso ao proxy de módulos do Go.
// A troca é só implementar Sink.Write — o resto do Core não muda.
package logger

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"eidolon/analytics-core/internal/models"
)

// New cria o logger de operação (eventos do processo: start, erros,
// falhas de dependência, decisões críticas). Formato JSON estruturado.
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}

// Sink recebe RequestLogs finalizados para persistência de longo prazo.
type Sink interface {
	Write(entry models.RequestLog)
	Close()
}

// AsyncSink faz buffering em um canal e escreve em uma goroutine dedicada,
// para nunca adicionar latência ao caminho crítico do request. Se o buffer
// encher, descarta o log mais antigo e loga um alerta — perder um log é
// preferível a bloquear tráfego só por causa de persistência de logs.
type AsyncSink struct {
	ch       chan models.RequestLog
	opLogger *slog.Logger
	write    func(models.RequestLog)
	wg       sync.WaitGroup
	closed   chan struct{}
}

// NewAsyncSink cria um sink assíncrono. `write` é a função real de
// persistência (stdout aqui; ClickHouse batch-insert em produção).
func NewAsyncSink(bufferSize int, opLogger *slog.Logger, write func(models.RequestLog)) *AsyncSink {
	s := &AsyncSink{
		ch:       make(chan models.RequestLog, bufferSize),
		opLogger: opLogger,
		write:    write,
		closed:   make(chan struct{}),
	}
	s.wg.Add(1)
	go s.loop()
	return s
}

func (s *AsyncSink) loop() {
	defer s.wg.Done()
	for {
		select {
		case entry := <-s.ch:
			s.write(entry)
		case <-s.closed:
			// drena o que sobrou antes de sair
			for {
				select {
				case entry := <-s.ch:
					s.write(entry)
				default:
					return
				}
			}
		}
	}
}

func (s *AsyncSink) Write(entry models.RequestLog) {
	select {
	case s.ch <- entry:
	default:
		// buffer cheio: nunca bloqueia o request por causa de log.
		s.opLogger.Warn("request log buffer cheio, log descartado",
			"correlation_id", entry.CorrelationID)
	}
}

func (s *AsyncSink) Close() {
	close(s.closed)
	s.wg.Wait()
}

// StdoutWriter é a implementação padrão de persistência (imprime JSON no
// stdout). Troque por um writer real de ClickHouse em produção.
func StdoutWriter(entry models.RequestLog) {
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	os.Stdout.Write(b)
	os.Stdout.Write([]byte("\n"))
}
