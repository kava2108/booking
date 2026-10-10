package service

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// RunSlotGenerator は枠の生成を即時に 1 回、その後は interval ごとに実行する。ctx が終了するまでブロックする。
// 生成は冪等（既存の枠は上書きしない）なので、重複実行・再起動しても安全。失敗はログに出して継続する。
func RunSlotGenerator(ctx context.Context, gen SlotGenerator, interval time.Duration, log zerolog.Logger) {
	run := func() {
		n, err := gen.GenerateUpcoming(ctx)
		if err != nil {
			log.Error().Err(err).Msg("slot generation failed")
			return
		}
		log.Info().Int("slots", n).Msg("slot generation finished")
	}
	run()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
