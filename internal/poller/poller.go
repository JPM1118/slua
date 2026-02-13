package poller

import (
	"context"
	"sync"
	"time"

	"github.com/JPM1118/slua/internal/sprites"
)

// Config holds poller configuration.
type Config struct {
	PollInterval   time.Duration
	ExecTimeout    time.Duration
	PromptPatterns []string
	MaxWorkers     int
}

// PollerUpdate is sent to the dashboard when polled states change.
type PollerUpdate struct {
	States map[string]SpriteState
}

// Poller runs background state detection for all Sprites.
type Poller struct {
	cli      sprites.SpriteSource
	cfg      Config
	script   string
	states   map[string]*SpriteState
	updateCh chan PollerUpdate
	triggerCh chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
}

// New creates a poller. Call Start() to begin polling.
func New(cli sprites.SpriteSource, cfg Config) *Poller {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 10
	}
	return &Poller{
		cli:       cli,
		cfg:       cfg,
		script:    BuildDetectionScript(cfg.PromptPatterns),
		states:    make(map[string]*SpriteState),
		updateCh:  make(chan PollerUpdate, 4),
		triggerCh: make(chan struct{}, 1),
	}
}

// Updates returns the channel that receives state updates.
func (p *Poller) Updates() <-chan PollerUpdate {
	return p.updateCh
}

// Start begins the polling loop. Blocks until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) {
	go p.run(ctx)
}

// TriggerNow requests an immediate poll cycle.
func (p *Poller) TriggerNow() {
	select {
	case p.triggerCh <- struct{}{}:
	default:
		// Already triggered, skip
	}
}

func (p *Poller) run(ctx context.Context) {
	// Initial poll
	p.pollCycle(ctx)

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollCycle(ctx)
		case <-p.triggerCh:
			p.pollCycle(ctx)
			// Reset ticker after manual trigger
			ticker.Reset(p.cfg.PollInterval)
		}
	}
}

func (p *Poller) pollCycle(ctx context.Context) {
	// Get current sprite list
	spriteList, err := p.cli.List(ctx)
	if err != nil {
		return // Silently skip cycle on list failure
	}

	// Filter to running Sprites only
	var running []sprites.Sprite
	for _, s := range spriteList {
		if s.Status == sprites.StatusWorking || s.Status == "RUNNING" || s.Status == "STARTED" {
			running = append(running, s)
		}
	}

	// If no running sprites, still update with any existing states cleared
	if len(running) == 0 {
		p.emitUpdate()
		return
	}

	// Poll in parallel with bounded concurrency
	type result struct {
		name   string
		status string
		detail string
		err    error
	}

	now := time.Now()
	results := make(chan result, len(running))
	sem := make(chan struct{}, p.cfg.MaxWorkers)

	var wg sync.WaitGroup
	for _, s := range running {
		name := s.Name

		p.mu.Lock()
		state, exists := p.states[name]
		if !exists {
			state = &SpriteState{Name: name}
			p.states[name] = state
		}
		shouldPoll := state.ShouldPoll(now)
		p.mu.Unlock()

		if !shouldPoll {
			continue
		}

		wg.Add(1)
		go func(spriteName string) {
			defer wg.Done()
			sem <- struct{}{} // acquire
			defer func() { <-sem }() // release

			execCtx, cancel := context.WithTimeout(ctx, p.cfg.ExecTimeout)
			defer cancel()

			status, detail, err := Detect(execCtx, p.cli, spriteName, p.script)
			results <- result{name: spriteName, status: status, detail: detail, err: err}
		}(name)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	p.mu.Lock()
	for r := range results {
		state := p.states[r.name]
		if r.err != nil {
			state.RecordFailure(p.cfg.PollInterval, now)
		} else {
			state.RecordSuccess(r.status, now)
			state.ErrorDetail = r.detail
		}
	}
	p.mu.Unlock()

	p.emitUpdate()
}

func (p *Poller) emitUpdate() {
	p.mu.Lock()
	snapshot := make(map[string]SpriteState, len(p.states))
	for k, v := range p.states {
		snapshot[k] = *v
	}
	p.mu.Unlock()

	// Non-blocking send — if channel is full, drop oldest
	select {
	case p.updateCh <- PollerUpdate{States: snapshot}:
	default:
		// Drain one and resend
		select {
		case <-p.updateCh:
		default:
		}
		select {
		case p.updateCh <- PollerUpdate{States: snapshot}:
		default:
		}
	}
}
