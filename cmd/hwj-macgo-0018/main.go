package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"hwj-macgo-0018/application"
)

func main() {
	selfCheck := flag.Bool("self-check", false, "执行离线自检")
	flag.Parse()
	if !*selfCheck {
		fmt.Println("idempotency credential core ready")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := application.NewService().SelfCheck(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "self-check failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("self-check passed")
}
