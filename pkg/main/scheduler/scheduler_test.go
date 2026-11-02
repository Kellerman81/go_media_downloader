package scheduler

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/robfig/cron/v3"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{in: "1d12h", want: 36 * time.Hour},
		{in: "2d", want: 48 * time.Hour},
		{in: "1w", want: 7 * 24 * time.Hour},
		{in: "1w1d", want: 8 * 24 * time.Hour},
		{in: "90m", want: 90 * time.Minute},
		{in: "1h30m", want: 90 * time.Minute},
		{in: " 3D ", want: 72 * time.Hour},
		{in: "10m", want: 10 * time.Minute},
		{in: "", err: true},
		{in: "12", err: true},
		{in: "abc", err: true},
		{in: "0d", err: true},
		{in: "-5m", err: true},
		{in: "12h1d", err: true},
	}

	for _, tt := range tests {
		got, err := parseInterval(tt.in)
		if (err != nil) != tt.err {
			t.Errorf("parseInterval(%q) err = %v, want err %v", tt.in, err, tt.err)
			continue
		}

		if !tt.err && got != tt.want {
			t.Errorf("parseInterval(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestIntervalToCron(t *testing.T) {
	parser := cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)

	convertible := []time.Duration{
		time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute,
		time.Hour, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour,
		24 * time.Hour, 7 * 24 * time.Hour,
	}

	for _, d := range convertible {
		expr, ok := intervalToCron(d, "job_"+d.String())
		if !ok {
			t.Errorf("%v: expected a cron expression", d)
			continue
		}

		sched, err := parser.Parse(expr)
		if err != nil {
			t.Errorf("%v -> %q does not parse: %v", d, expr, err)
			continue
		}

		// Consecutive runs must be exactly the interval apart.
		start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		first := sched.Next(start)
		second := sched.Next(first)
		third := sched.Next(second)

		if second.Sub(first) != d || third.Sub(second) != d {
			t.Errorf("%v -> %q fires at %v, %v, %v", d, expr, first, second, third)
		}
	}

	for _, d := range []time.Duration{
		30 * time.Second, 90 * time.Minute, 7 * time.Minute, 36 * time.Hour,
		5 * time.Hour, 48 * time.Hour, 3 * 24 * time.Hour,
	} {
		if expr, ok := intervalToCron(d, "x"); ok {
			t.Errorf("%v should fall back to an interval timer, got %q", d, expr)
		}
	}

	a, _ := intervalToCron(time.Hour, "searchmissinginc_movie_EN")
	b, _ := intervalToCron(time.Hour, "searchmissinginc_movie_EN")
	if a != b {
		t.Errorf("offset not stable: %q vs %q", a, b)
	}
}

// TestApplyKeepsUnchangedSchedules: a reload must leave unchanged schedules
// (and their ticker phase) alone and replace only what changed.
func TestApplyKeepsUnchangedSchedules(t *testing.T) {
	syncops.InitSyncOps()
	worker.RegisterWorkerSyncMaps()
	worker.CreateCronWorker()

	a := scheduleSpec{interval: "1h", name: "apply_a", queue: worker.QueueData, jobname: "x"}
	b := scheduleSpec{cron: "0 3 * * *", name: "apply_b", queue: worker.QueueSearch, jobname: "x"}

	apply([]scheduleSpec{a, b})
	t.Cleanup(func() { apply(nil) })

	idA, idB := active["apply_a"].id, active["apply_b"].id
	if idA == 0 || idB == 0 {
		t.Fatalf("schedules not dispatched: %+v", active)
	}

	b.cron = "0 4 * * *"
	apply([]scheduleSpec{a, b})

	if active["apply_a"].id != idA {
		t.Error("unchanged schedule was re-created")
	}

	if active["apply_b"].id == idB {
		t.Error("changed schedule was not replaced")
	}

	if _, ok := worker.GetSchedules()[idB]; ok {
		t.Error("old schedule still registered")
	}

	apply([]scheduleSpec{a})

	if _, ok := active["apply_b"]; ok {
		t.Error("removed schedule still active")
	}

	if n := len(worker.GetSchedules()); n != 1 {
		t.Errorf("%d schedules registered, want 1", n)
	}
}

// TestRefreshJobsHaveScheduleFields: every refresh* job a media handler
// registers must be scheduled and read its own Default-template fields.
func TestRefreshJobsHaveScheduleFields(t *testing.T) {
	def := &config.SchedulerConfig{}
	v := reflect.ValueOf(def).Elem()

	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Type.Kind() == reflect.String {
			v.Field(i).SetString(f.Name)
		}
	}

	cfgp := &config.MediaTypeConfig{CfgScheduler: &config.SchedulerConfig{}}
	seen := map[string]string{}

	for _, typ := range []uint{
		config.MediaTypeMovie, config.MediaTypeSeries, config.MediaTypeBook,
		config.MediaTypeAudiobook, config.MediaTypeMusic,
	} {
		h := mediatype.Get(typ)
		if h == nil {
			t.Fatalf("no handler for media type %d", typ)
		}

		for _, pair := range h.GetSchedulerJobNames() {
			job := pair[0]
			if !strings.HasPrefix(job, "refresh") {
				continue
			}

			if !slices.Contains(mediaJobNames, job) {
				t.Errorf("%s is not in mediaJobNames", job)
			}

			if q := mediaQueue(job); q != worker.QueueFeeds {
				t.Errorf("%s queue = %s, want %s", job, q, worker.QueueFeeds)
			}

			interval, cron := mediaSchedule(cfgp, def, h, job)
			if interval == "" || cron == "" {
				t.Errorf("%s: no schedule fields mapped (%q, %q)", job, interval, cron)

				continue
			}

			if !strings.HasPrefix(interval, "Interval") || !strings.HasPrefix(cron, "Cron") ||
				strings.TrimPrefix(interval, "Interval") != strings.TrimPrefix(cron, "Cron") {
				t.Errorf("%s: mismatched fields %s / %s", job, interval, cron)
			}

			if other, ok := seen[interval]; ok {
				t.Errorf("%s and %s share field %s", job, other, interval)
			}

			seen[interval] = job
		}
	}

	if len(seen) != 10 {
		t.Errorf("%d refresh jobs mapped, want 10", len(seen))
	}
}
