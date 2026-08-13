package main

import (
	"context"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

func (s *AppService) InspectWSLAwareness() core.WSLAwareness {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return s.wslInspector.Inspect(ctx)
}
