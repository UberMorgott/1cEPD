package updater

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"partnerops/internal/selfupdate"
	"partnerops/internal/settings"
)

type fakeRelease struct {
	version   string
	installed atomic.Int32
	err       error
}

func (r *fakeRelease) Version() string { return r.version }

func (r *fakeRelease) Install(_ context.Context, _ string, progress selfupdate.Progress) error {
	if progress != nil {
		progress(10, 10)
	}
	if r.err != nil {
		return r.err
	}
	r.installed.Add(1)
	return nil
}

type memPrefs struct{ p settings.UpdatePrefs }

func (m *memPrefs) UpdatePrefs(context.Context) (settings.UpdatePrefs, error) { return m.p, nil }
func (m *memPrefs) SaveUpdatePrefs(_ context.Context, p settings.UpdatePrefs) error {
	m.p = p
	return nil
}

func newTest(version string, rel *fakeRelease, relaunched, quit *atomic.Int32) *Updater {
	u := New(version, `C:\app\pult-epd.exe`, &memPrefs{p: settings.UpdatePrefs{Check: true, Install: false}},
		func() error { relaunched.Add(1); return nil }, func() { quit.Add(1) })
	return u.WithLatest(func(_ context.Context, current string) (Release, bool, error) {
		if rel == nil {
			return nil, false, nil
		}
		return rel, newer(rel.version, current), nil
	})
}

// newer повторяет правило selfupdate: строго новее и обе версии читаемы.
func newer(a, b string) bool {
	return selfupdate.Valid(a) && selfupdate.Valid(b) && a != b && a > b
}

func TestCheckFindsNewerAndInstallRestarts(t *testing.T) {
	var relaunched, quit atomic.Int32
	rel := &fakeRelease{version: "0.1.0"}
	u := newTest("0.0.1", rel, &relaunched, &quit)

	st := u.Check(context.Background())
	if !st.Available || st.Latest != "0.1.0" || st.Current != "0.0.1" {
		t.Fatalf("Check = %+v", st)
	}
	st = u.Install(context.Background())
	if !st.Restarting || rel.installed.Load() != 1 {
		t.Fatalf("Install = %+v, installed %d", st, rel.installed.Load())
	}
	deadline := time.Now().Add(3 * time.Second)
	for (relaunched.Load() == 0 || quit.Load() == 0) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if relaunched.Load() != 1 || quit.Load() != 1 {
		t.Errorf("relaunch %d, quit %d, want 1 and 1", relaunched.Load(), quit.Load())
	}
}

func TestUpToDate(t *testing.T) {
	var relaunched, quit atomic.Int32
	u := newTest("0.1.0", &fakeRelease{version: "0.1.0"}, &relaunched, &quit)
	st := u.Check(context.Background())
	if st.Available || st.Text != "Установлена последняя версия." {
		t.Fatalf("Check = %+v", st)
	}
	if st = u.Install(context.Background()); st.Restarting {
		t.Fatal("ставим ту же версию")
	}
}

func TestInstallFailureReported(t *testing.T) {
	var relaunched, quit atomic.Int32
	u := newTest("0.0.1", &fakeRelease{version: "0.1.0", err: errors.New("checksum mismatch")}, &relaunched, &quit)
	st := u.Install(context.Background())
	if !st.Failed || st.Restarting || st.Busy {
		t.Fatalf("Install = %+v", st)
	}
	if relaunched.Load() != 0 {
		t.Error("перезапуск после неудачной установки")
	}
}

func TestDevBuildDisabled(t *testing.T) {
	var relaunched, quit atomic.Int32
	u := newTest("dev", &fakeRelease{version: "0.1.0"}, &relaunched, &quit)
	if st := u.Check(context.Background()); st.Enabled || st.Available {
		t.Fatalf("dev Check = %+v", st)
	}
}

func TestPrefsRoundTrip(t *testing.T) {
	var relaunched, quit atomic.Int32
	u := newTest("0.0.1", nil, &relaunched, &quit)
	if err := u.SetPrefs(context.Background(), settings.UpdatePrefs{Check: false, Install: true}); err != nil {
		t.Fatal(err)
	}
	if st := u.Status(context.Background()); st.AutoCheck || !st.AutoInstall {
		t.Fatalf("Status = %+v", st)
	}
}
