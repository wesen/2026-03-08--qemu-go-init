package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-go-golems/bobatea/pkg/eventbus"
	"github.com/go-go-golems/bobatea/pkg/repl"
	jsadapter "github.com/go-go-golems/go-go-goja/pkg/repl/adapters/bobatea"
)

func main() {
	logger := log.New(os.Stderr, "js-repl-probe: ", log.LstdFlags|log.Lmicroseconds)

	evaluator, err := jsadapter.NewJavaScriptEvaluatorWithDefaults()
	if err != nil {
		logger.Fatalf("create evaluator: %v", err)
	}
	defer func() {
		if closeErr := evaluator.Close(); closeErr != nil {
			logger.Printf("close evaluator: %v", closeErr)
		}
	}()

	bus, err := eventbus.NewInMemoryBus()
	if err != nil {
		logger.Fatalf("create bus: %v", err)
	}
	repl.RegisterReplToTimelineTransformer(bus)

	cfg := repl.DefaultConfig()
	cfg.Title = "ticket-007 probe"
	cfg.Placeholder = "2 + 2"
	model := repl.NewModel(evaluator, cfg, bus.Publisher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if runErr := bus.Run(ctx); runErr != nil {
			logger.Printf("bus exited: %v", runErr)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	fmt.Printf("evaluator=%s prompt=%s multiline=%t\n", evaluator.GetName(), evaluator.GetPrompt(), evaluator.SupportsMultiline())
	fmt.Printf("repl-model=%T title=%q\n", model, cfg.Title)
	fmt.Println("events:")
	err = evaluator.EvaluateStream(ctx, "const nums = [1,2,3]; nums.reduce((a, b) => a + b, 0)", func(event repl.Event) {
		fmt.Printf("- kind=%s props=%v\n", event.Kind, event.Props)
	})
	if err != nil {
		logger.Fatalf("evaluate stream: %v", err)
	}
}
