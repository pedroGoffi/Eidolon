package security

import (
	"sync"
	"time"
)

// AnomalyDetector compara a taxa de requests recente de uma chave (IP ou
// IP+rota) contra uma baseline móvel simples (média das últimas N janelas).
// Um desvio acima do threshold é reportado como spike.
//
// Implementação propositalmente simples (média móvel), suficiente para
// detectar floods óbvios. Evoluir para EWMA ou modelo adaptativo é uma
// melhoria futura natural (ver seção 9 do blueprint).
type AnomalyDetector struct {
	mu          sync.Mutex
	windowSize  time.Duration
	historySize int
	threshold   float64 // multiplicador sobre a média (ex: 5.0 = 5x a baseline)
	series      map[string]*keySeries
}

type keySeries struct {
	counts       []int
	currentCount int
	windowStart  time.Time
}

func NewAnomalyDetector(windowSize time.Duration, historySize int, threshold float64) *AnomalyDetector {
	return &AnomalyDetector{
		windowSize:  windowSize,
		historySize: historySize,
		threshold:   threshold,
		series:      make(map[string]*keySeries),
	}
}

// Record registra uma ocorrência para `key` e retorna true se a taxa
// atual excede o threshold em relação à baseline histórica.
func (a *AnomalyDetector) Record(key string) (isSpike bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	s, exists := a.series[key]
	if !exists {
		a.series[key] = &keySeries{windowStart: now, currentCount: 1}
		return false
	}

	if now.Sub(s.windowStart) >= a.windowSize {
		// fecha a janela atual, empurra pro histórico
		s.counts = append(s.counts, s.currentCount)
		if len(s.counts) > a.historySize {
			s.counts = s.counts[len(s.counts)-a.historySize:]
		}
		s.currentCount = 1
		s.windowStart = now
		return false // não avalia spike na primeira contagem da nova janela
	}

	s.currentCount++

	if len(s.counts) == 0 {
		return false // sem baseline ainda
	}

	sum := 0
	for _, c := range s.counts {
		sum += c
	}
	avg := float64(sum) / float64(len(s.counts))
	if avg == 0 {
		return false
	}

	return float64(s.currentCount) > avg*a.threshold
}
