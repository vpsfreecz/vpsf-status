package main

import (
	"os"
	"testing"
)

func TestCheckNoticeFileOnceUpdatesMetric(t *testing.T) {
	_, st, cfg := newTestApplication(t)

	writeNotice(t, cfg, "<p>Notice</p>")
	if err := checkNoticeFileOnce(st, cfg.NoticeFile); err != nil {
		t.Fatalf("checkNoticeFileOnce: %v", err)
	}
	if got := gaugeValue(t, st.Exporter.notice); got != 1 {
		t.Fatalf("notice gauge = %v, want 1", got)
	}

	if err := os.Remove(cfg.NoticeFile); err != nil {
		t.Fatalf("remove notice: %v", err)
	}
	if err := checkNoticeFileOnce(st, cfg.NoticeFile); err != nil {
		t.Fatalf("checkNoticeFileOnce missing notice: %v", err)
	}
	if got := gaugeValue(t, st.Exporter.notice); got != 0 {
		t.Fatalf("notice gauge = %v, want 0", got)
	}

	if err := checkNoticeFileOnce(st, t.TempDir()); err == nil {
		t.Fatal("checkNoticeFileOnce returned nil error for directory notice path")
	}
	if got := gaugeValue(t, st.Exporter.notice); got != 0 {
		t.Fatalf("notice gauge = %v, want 0", got)
	}
}
