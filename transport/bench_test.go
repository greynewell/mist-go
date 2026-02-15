package transport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greynewell/mist-go/protocol"
)

func BenchmarkChannelSendReceive(b *testing.B) {
	ch := NewChannel(b.N)
	ctx := context.Background()
	msg, _ := protocol.New(protocol.SourceMatchSpec, protocol.TypeHealthPing, protocol.HealthPing{From: "bench"})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ch.Send(ctx, msg)
	}
	for i := 0; i < b.N; i++ {
		_, _ = ch.Receive(ctx)
	}
}

func BenchmarkChannelPairRoundtrip(b *testing.B) {
	a, bCh := NewChannelPair(1)
	ctx := context.Background()
	msg, _ := protocol.New(protocol.SourceMatchSpec, protocol.TypeHealthPing, protocol.HealthPing{From: "bench"})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = a.Send(ctx, msg)
		_, _ = bCh.Receive(ctx)
	}
}

func BenchmarkChannelSendReceive_LargeMsg(b *testing.B) {
	ch := NewChannel(b.N)
	ctx := context.Background()
	msg, _ := protocol.New(protocol.SourceInferMux, protocol.TypeInferResponse, protocol.InferResponse{
		Content: strings.Repeat("benchmark large payload ", 1000),
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ch.Send(ctx, msg)
	}
	for i := 0; i < b.N; i++ {
		_, _ = ch.Receive(ctx)
	}
}

func BenchmarkFileSend(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.jsonl")

	ft, _ := NewFile(path)
	defer ft.Close()

	ctx := context.Background()
	msg, _ := protocol.New(protocol.SourceMatchSpec, protocol.TypeHealthPing, protocol.HealthPing{From: "bench"})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ft.Send(ctx, msg)
	}

	info, _ := os.Stat(path)
	b.SetBytes(info.Size() / int64(b.N))
}

func BenchmarkFileReceive(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.jsonl")

	// Pre-write messages.
	ft, _ := NewFile(path)
	ctx := context.Background()
	msg, _ := protocol.New(protocol.SourceMatchSpec, protocol.TypeHealthPing, protocol.HealthPing{From: "bench"})
	for i := 0; i < b.N; i++ {
		ft.Send(ctx, msg)
	}
	ft.Close()

	// Benchmark reading.
	ft2, _ := NewFile(path)
	defer ft2.Close()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ft2.Receive(ctx)
	}
}

func BenchmarkFileSendLargeMsg(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench_large.jsonl")

	ft, _ := NewFile(path)
	defer ft.Close()

	ctx := context.Background()
	msg, _ := protocol.New(protocol.SourceInferMux, protocol.TypeInferResponse, protocol.InferResponse{
		Content: strings.Repeat("large payload content here ", 500),
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ft.Send(ctx, msg)
	}
}
