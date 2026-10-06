package service

import (
	"strings"
	"testing"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

func TestFindVanishedSuspectsThenConfirms(t *testing.T) {
	prev := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	older := prev.Add(-24 * time.Hour)
	known := []store.IdentifierRecord{
		{EDOID: "A", INN: "7700000001", LastSeenAt: prev, FirstSeenAt: older},
		// Пропал ещё в прошлом снимке: второй подряд — подтверждение.
		{EDOID: "B", INN: "7700000002", Login: "b", LastSeenAt: older, FirstSeenAt: older},
		// Был в прошлом снимке: пока подозрение.
		{EDOID: "C", INN: "7700000003", Login: "c", LastSeenAt: prev, FirstSeenAt: older},
	}
	rows := []partner.EDOBillingRow{{EDOID: "A", INN: "7700000001"}}

	found := FindVanished(rows, known, "2026-08")
	byID := map[string]Anomaly{}
	for _, a := range found {
		byID[a.EDOID] = a
	}
	if len(found) != 2 {
		t.Fatalf("находок %d, ожидали 2: %+v", len(found), found)
	}
	if b := byID["B"]; b.Kind != KindDisappeared || b.Confidence != ConfidenceHigh {
		t.Errorf("B: %+v, ожидали подтверждённое исчезновение", b)
	}
	// У B владельца не было: в тексте не должно остаться пустых кавычек.
	if b := byID["B"]; strings.Contains(b.Details, "«»") || !strings.Contains(b.Details, "владельца не было") {
		t.Errorf("B: подробности %q", b.Details)
	}
	c := byID["C"]
	if c.Kind != KindDisappeared || c.Confidence != ConfidenceLow {
		t.Errorf("C: %+v, ожидали подозрение", c)
	}

	// Следующий снимок: C не вернулся — тот же отпечаток, но уже подтверждение.
	later := prev.Add(24 * time.Hour)
	known[0].LastSeenAt = later
	again := FindVanished(rows, known, "2026-08")
	for _, a := range again {
		if a.EDOID == "C" && (a.Confidence != ConfidenceHigh || a.StateFingerprint != c.StateFingerprint) {
			t.Errorf("C во втором снимке: %+v, ожидали подтверждение с прежним отпечатком", a)
		}
	}
}

func TestFindVanishedReportsReplacementOnce(t *testing.T) {
	prev := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	known := []store.IdentifierRecord{
		{EDOID: "OLD", INN: "7700000001", Login: "x", LastSeenAt: prev, FirstSeenAt: prev},
		{EDOID: "KEEP", INN: "7700000009", Login: "y", LastSeenAt: prev, FirstSeenAt: prev},
	}
	rows := []partner.EDOBillingRow{
		{EDOID: "KEEP", INN: "7700000009", Login: "y"},
		// Новый у того же логина, ИНН другой — всё равно замена.
		{EDOID: "NEW", INN: "7700000005", Login: "X"},
	}
	found := FindVanished(rows, known, "2026-08")
	if len(found) != 1 || found[0].Kind != KindReplaced || found[0].EDOID != "NEW" {
		t.Fatalf("находки %+v, ожидали одну замену по NEW", found)
	}

	// Законный второй идентификатор, известный давно, заменой не считается.
	known = append(known, store.IdentifierRecord{EDOID: "NEW", INN: "7700000005", Login: "x",
		LastSeenAt: prev, FirstSeenAt: prev.Add(-time.Hour)})
	found = FindVanished(rows, known, "2026-08")
	if len(found) != 1 || found[0].Kind != KindDisappeared {
		t.Errorf("находки %+v, ожидали исчезновение OLD", found)
	}
}

func TestSuppressedByTopologyOnlyStructuralKinds(t *testing.T) {
	topology := map[string]string{TopologyKey("7700000001", "770001001", "A"): "второй вид деятельности"}
	if purpose, ok := SuppressedByTopology(KindOrphanIdle, "7700000001", "770001001", "A", topology); !ok || purpose == "" {
		t.Error("orphan_idle по связи из топологии должен гаситься")
	}
	for _, kind := range []string{KindOrphanWithTraffic, KindOwnerLost, KindDisappeared} {
		if _, ok := SuppressedByTopology(kind, "7700000001", "770001001", "A", topology); ok {
			t.Errorf("%s топология гасить не должна", kind)
		}
	}
	if _, ok := SuppressedByTopology(KindOrphanIdle, "7700000001", "", "A", topology); ok {
		t.Error("другой КПП — другая связь")
	}
}
